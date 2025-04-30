import os
import requests
from flask import Flask, jsonify, request, abort
from flask_cors import CORS
from dotenv import load_dotenv
import psycopg2
from binance_client import get_asset_balance, place_market_order, place_limit_order

load_dotenv()

# Database configuration
DB_HOST = os.getenv("DB_HOST")
DB_NAME = os.getenv("DB_NAME")
DB_USER = os.getenv("DB_USER")
DB_PASSWORD = os.getenv("DB_PASSWORD")
DB_PORT = os.getenv("DB_PORT", 5432)

conn = psycopg2.connect(
    host=DB_HOST,
    dbname=DB_NAME,
    user=DB_USER,
    password=DB_PASSWORD,
    port=DB_PORT
)
conn.autocommit = True

app = Flask(__name__)
CORS(app, resources={r"/*": {"origins": ["http://localhost:8080", "https://localhost:8080"]}})

@app.route("/test", methods=["GET"])
def test():
    return jsonify({"message": "API is working!"}), 200

@app.route("/registerAlert", methods=["POST"])
def register_alert():
    data = request.get_json()
    email = data.get("email")
    symbolAndId = data.get("symbolAndId")
    threshold = data.get("threshold")
    usingUsd = data.get("usingUsd")
    symbol, crypto_id = symbolAndId.split(" - ")

    try:
        response = requests.get(
            f"https://api.coingecko.com/api/v3/simple/price?ids={crypto_id}&vs_currencies=usd,brl"
        )
        response.raise_for_status()
        price_data = response.json()
        usd = float(price_data[crypto_id]['usd'])
        brl = float(price_data[crypto_id]['brl'])
    except Exception as e:
        return jsonify({"error": "CoinGecko Service Unavailable"}), 503

    current_value = usd
    converted_threshold = float(threshold)
    if not usingUsd:
        converted_threshold = (converted_threshold * current_value) / brl

    greater_than_current = converted_threshold >= current_value

    cur = conn.cursor()
    cur.execute(
        "INSERT INTO cripto_email (email) VALUES (%s) ON CONFLICT (email) DO NOTHING",
        (email,)
    )
    cur.execute(
        "INSERT INTO cripto_currency (symbol, cryptoId) VALUES (%s, %s) ON CONFLICT (symbol) DO NOTHING",
        (symbol, crypto_id)
    )
    cur.execute("""
        INSERT INTO cripto_threshold (id_email, id_cripto, threshold, greaterThanCurrent, created_at)
        VALUES (
            (SELECT id FROM cripto_email WHERE email = %s),
            (SELECT id FROM cripto_currency WHERE symbol = %s),
            %s, %s, NOW()
        )
    """, (email, symbol, converted_threshold, greater_than_current))
    cur.close()
    return jsonify({"success": True}), 201

@app.route("/clearAlerts", methods=["POST"])
def clear_alerts():
    cur = conn.cursor()
    cur.execute("""
        DELETE FROM cripto_threshold 
        WHERE created_at IS NULL OR created_at < (NOW() - INTERVAL '1 WEEK')
    """)
    cur.execute("""
        WITH repeated_alerts AS (
            SELECT id, ROW_NUMBER() OVER (PARTITION BY id_cripto, id_email, greaterthancurrent ORDER BY created_at DESC) AS rn
            FROM cripto_threshold
        )
        DELETE FROM cripto_threshold
        WHERE id IN (SELECT id FROM repeated_alerts WHERE rn > 1)
    """)
    cur.close()
    return jsonify({"success": True}), 201

@app.route("/clearAlertById", methods=["POST"])
def clear_alert_by_id():
    data = request.get_json()
    alert_id = data.get("id")
    cur = conn.cursor()
    cur.execute("DELETE FROM cripto_threshold WHERE id = %s", (alert_id,))
    cur.close()
    return jsonify({"success": True}), 201

@app.route("/getCryptos", methods=["GET"])
def get_cryptos():
    cur = conn.cursor()
    cur.execute("SELECT id, cryptoId AS cryptoid FROM cripto_currency")
    rows = cur.fetchall()
    cur.close()
    
    if not rows:
        return jsonify({"message": "No cryptocurrency found."}), 204

    cryptos = [{"id": row[0], "cryptoid": row[1]} for row in rows]
    return jsonify(cryptos), 200

@app.route("/reachedThresholds", methods=["GET"])
def reached_thresholds():
    crypto_id = request.args.get("id")
    crypto_value = float(request.args.get("cryptoValue"))
    cur = conn.cursor()
    cur.execute("""
        SELECT ct.id, ct.threshold, ct.greaterthancurrent, ce.email
        FROM cripto_threshold ct
        INNER JOIN cripto_currency cc ON cc.id = ct.id_cripto
        INNER JOIN cripto_email ce ON ce.id = ct.id_email
        WHERE ct.id_cripto = %s
        AND CASE 
            WHEN ct.greaterthancurrent = TRUE THEN ct.threshold <= %s
            ELSE ct.threshold >= %s
        END
    """, (crypto_id, crypto_value, crypto_value))
    rows = cur.fetchall()
    thresholds = [{"id": row[0], "threshold": row[1], "greaterthancurrent": row[2], "email": row[3]} for row in rows]
    cur.close()
    return jsonify(thresholds), 200

@app.route("/asset-balance", methods=["GET"])
def asset_balance():
    asset = request.args.get("asset", "BTC").upper()
    print(f"[DEBUG] Testing Binance connection for asset: {asset}")
    result = get_asset_balance(asset)
    status = 200 if result.get("status") == "success" else 500
    return jsonify(result), status
    
@app.route("/order", methods=["POST"])
def order():
    data = request.get_json() or {}
    symbol = data.get("symbol")
    side   = data.get("side")
    qty_in = data.get("quantity")
    quote  = data.get("quoteOrderQty")

    if not symbol or not side or (qty_in is None and quote is None):
        abort(400, "'symbol', 'side' and either 'quantity' or 'quoteOrderQty' are required")

    if quote is not None:
        try:
            used_amount = float(quote)
        except ValueError:
            abort(400, "'quoteOrderQty' must be a number")
        result = place_market_order(symbol, side, quoteOrderQty=used_amount)
    else:
        try:
            used_amount = float(qty_in)
        except ValueError:
            abort(400, "'quantity' must be a number")
        result = place_market_order(symbol, side, quantity=used_amount)

    if result.get("status") == "success":
        order_data = result["order"]
        cur = conn.cursor()
        cur.execute("""
            INSERT INTO trades
              (order_id, on_testnet, client_order_id, symbol, side,
               qty, quote_qty, price, status)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
        """, (
            order_data["orderId"],
            result.get("onTestnet", False),
            order_data["clientOrderId"],
            order_data["symbol"],
            order_data["side"],
            order_data["executedQty"],
            order_data["cummulativeQuoteQty"],
            order_data.get("fills", [{}])[0].get("price"),
            order_data["status"]
        ))
        conn.commit()
        cur.close()
        return jsonify(result), 200
    else:
        status_code = 400 if "minQty" in result else 500
        return jsonify(result), status_code

# NEW endpoint for limit orders (sells)
@app.route("/limit-order", methods=["POST"])
def limit_order():
    data = request.get_json() or {}
    symbol = data.get("symbol")
    side   = data.get("side")
    qty    = data.get("quantity")
    price  = data.get("price")

    if not symbol or not side or not qty or not price:
        abort(400, "'symbol', 'side', 'quantity' and 'price' are required for limit orders")
    try:
        qty = float(qty)
        price = float(price)
    except ValueError:
        abort(400, "'quantity' and 'price' must be numeric")

    result = place_limit_order(symbol, side, quantity=qty, price=price)

    if result.get("status") == "success":
        order_data = result["order"]
        cur = conn.cursor()
        cur.execute("""
            INSERT INTO trades
              (order_id, on_testnet, client_order_id, symbol, side,
               qty, quote_qty, price, status)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
        """, (
            order_data["orderId"],
            result.get("onTestnet", False),
            order_data["clientOrderId"],
            order_data["symbol"],
            order_data["side"],
            order_data["origQty"],
            order_data.get("origQuoteOrderQty", 0),
            order_data.get("price"),
            order_data["status"]
        ))
        conn.commit()
        cur.close()
        return jsonify(result), 200
    else:
        return jsonify(result), 500

if __name__ == "__main__":
    port = int(os.getenv("PORT", 5000))
    app.run(host="0.0.0.0", port=port)
