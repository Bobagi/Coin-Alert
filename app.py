import os
import requests
from flask import Flask, jsonify, request, abort
from flask_cors import CORS
from dotenv import load_dotenv
import psycopg2
from binance_client import get_asset_balance, place_market_order, place_limit_order
from logger_config import setup_logger
from routes.dashboard import dashboard_bp

load_dotenv()

logger = setup_logger("api-service")

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

app.register_blueprint(dashboard_bp)

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
        logger.error(f"Failed to fetch price from CoinGecko: {e}")
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
        "INSERT INTO cripto_currency (symbol, crypto_id) VALUES (%s, %s) ON CONFLICT (symbol) DO NOTHING",
        (symbol, crypto_id)
    )
    cur.execute("""
        INSERT INTO cripto_threshold (id_email, id_cripto, threshold, greater_than_current, created_at)
        VALUES (
            (SELECT id FROM cripto_email WHERE email = %s),
            (SELECT id FROM cripto_currency WHERE symbol = %s),
            %s, %s, NOW()
        )
    """, (email, symbol, converted_threshold, greater_than_current))
    cur.close()
    logger.info(f"Registered alert for {email} on {symbol} with threshold {converted_threshold}")
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
            SELECT id, ROW_NUMBER() OVER (PARTITION BY id_cripto, id_email, greater_than_current ORDER BY created_at DESC) AS rn
            FROM cripto_threshold
        )
        DELETE FROM cripto_threshold
        WHERE id IN (SELECT id FROM repeated_alerts WHERE rn > 1)
    """)
    cur.close()
    logger.info("Old and duplicate alerts cleared")
    return jsonify({"success": True}), 201

@app.route("/clearAlertById", methods=["POST"])
def clear_alert_by_id():
    data = request.get_json()
    alert_id = data.get("id")
    cur = conn.cursor()
    cur.execute("DELETE FROM cripto_threshold WHERE id = %s", (alert_id,))
    cur.close()
    logger.info(f"Alert {alert_id} cleared")
    return jsonify({"success": True}), 201

@app.route("/getCryptos", methods=["GET"])
def get_cryptos():
    cur = conn.cursor()
    cur.execute("SELECT id, crypto_id FROM cripto_currency")
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
        SELECT ct.id, ct.threshold, ct.greater_than_current, ce.email
        FROM cripto_threshold ct
        INNER JOIN cripto_currency cc ON cc.id = ct.id_cripto
        INNER JOIN cripto_email ce ON ce.id = ct.id_email
        WHERE ct.id_cripto = %s
        AND CASE 
            WHEN ct.greater_than_current = TRUE THEN ct.threshold <= %s
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
    user_id = request.args.get("userId")

    if not user_id:
        return jsonify({"error": "'userId' is required"}), 400

    try:
        user_id = int(user_id)
    except ValueError:
        return jsonify({"error": "'userId' must be an integer"}), 400

    logger.info(f"Requesting balance for asset: {asset} (user_id={user_id})")
    result = get_asset_balance(asset, user_id=user_id)
    status = 200 if result.get("status") == "success" else 500
    return jsonify(result), status

@app.route("/order", methods=["POST"])
def order():
    data = request.get_json() or {}
    symbol = data.get("symbol")
    side = data.get("side")
    qty_in = data.get("quantity")
    quote = data.get("quoteOrderQty")
    operation_type = data.get("operationType")
    user_id = data.get("userId")

    if not symbol or not side or (qty_in is None and quote is None) or user_id is None:
        abort(400, "'symbol', 'side', 'userId' and either 'quantity' or 'quoteOrderQty' are required")

    try:
        user_id = int(user_id)
        if quote is not None:
            used_amount = float(quote)
            result = place_market_order(symbol, side, user_id=user_id, quoteOrderQty=used_amount)
        else:
            used_amount = float(qty_in)
            result = place_market_order(symbol, side, user_id=user_id, quantity=used_amount)
    except ValueError:
        abort(400, "'quantity' or 'quoteOrderQty' must be a number")

    if result.get("status") == "success":
        order_data = result["order"]
        cur = conn.cursor()
        cur.execute("""
            INSERT INTO trades
              (order_id, on_testnet, client_order_id, symbol, side,
               qty, quote_qty, price, status, operation_type)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
        """, (
            order_data["orderId"],
            result.get("onTestnet", False),
            order_data["clientOrderId"],
            order_data["symbol"],
            order_data["side"],
            order_data["executedQty"],
            order_data["cummulativeQuoteQty"],
            order_data.get("fills", [{}])[0].get("price"),
            order_data["status"],
            operation_type
        ))
        conn.commit()
        cur.close()
        logger.info(f"Market order placed: {order_data['orderId']}")
        return jsonify(result), 200
    else:
        logger.error(f"Order failed: {result}")
        status_code = 400 if "minQty" in result else 500
        return jsonify(result), status_code


@app.route("/limit-order", methods=["POST"])
def limit_order():
    data = request.get_json() or {}
    symbol = data.get("symbol")
    side = data.get("side")
    qty = data.get("quantity")
    price = data.get("price")
    user_id = data.get("userId")
    operation_type = data.get("operationType")

    if not symbol or not side or not qty or not price or user_id is None:
        abort(400, "'symbol', 'side', 'quantity', 'price' and 'userId' are required for limit orders")

    try:
        qty = float(qty)
        price = float(price)
        user_id = int(user_id)
    except ValueError:
        abort(400, "'quantity' and 'price' must be numeric")

    result = place_limit_order(symbol, side, quantity=qty, price=price, user_id=user_id)

    if result.get("status") == "success":
        order_data = result["order"]
        cur = conn.cursor()
        cur.execute("""
            INSERT INTO trades
              (order_id, on_testnet, client_order_id, symbol, side,
               qty, quote_qty, price, status, operation_type)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
        """, (
            order_data["orderId"],
            result.get("onTestnet", False),
            order_data["clientOrderId"],
            order_data["symbol"],
            order_data["side"],
            order_data["origQty"],
            order_data.get("origQuoteOrderQty", 0),
            order_data.get("price"),
            order_data["status"],
            operation_type
        ))
        conn.commit()
        cur.close()
        logger.info(f"Limit order placed: {order_data['orderId']}")
        return jsonify(result), 200
    else:
        logger.error(f"Limit order failed: {result}")
        return jsonify(result), 500

if __name__ == "__main__":
    port = int(os.getenv("API_PORT", 5020))
    logger.info(f"Starting API service on port {port}")
    logger.info(
        "Dashboard available at http://localhost:%s/dashboard", port
    )
    app.run(host="0.0.0.0", port=port)
