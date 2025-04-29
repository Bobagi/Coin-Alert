#!/usr/bin/env python3
import os, sys
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import time
import requests
import psycopg2
from dotenv import load_dotenv
from decimal import Decimal, ROUND_DOWN
from binance_client import get_binance_client

dotenv_path = os.path.join(os.path.dirname(__file__), '..', '.env')
load_dotenv(dotenv_path)

DB_CONFIG = {
    'host': os.getenv('DB_HOST'),
    'dbname': os.getenv('DB_NAME'),
    'user': os.getenv('DB_USER'),
    'password': os.getenv('DB_PASSWORD'),
    'port': os.getenv('DB_PORT', 5432)
}
API_URL = os.getenv('API_URL', 'http://api:5000')
TRADE_SYMBOL = os.getenv('TRADE_SYMBOL', 'BTCUSDT')
SELL_THRESHOLD_PCT = float(os.getenv('SELL_THRESHOLD_PCT', 1.0))
PURCHASE_LIMIT_QUOTE = float(os.getenv('PURCHASE_LIMIT_QUOTE', 100.0))
POLL_INTERVAL = int(os.getenv('POLL_INTERVAL_SECONDS', 60))


def get_db_connection():
    conn = psycopg2.connect(**DB_CONFIG)
    conn.autocommit = True
    return conn

def get_open_buys(conn):
    with conn.cursor() as cur:
        cur.execute("""
            SELECT t.order_id, t.symbol, t.qty::float, t.price::float
            FROM trades t
            JOIN auto_positions ap ON ap.trade_id = t.order_id
            WHERE t.side = 'BUY'
              AND t.symbol = %s
              AND ap.sell_date IS NULL
        """, (TRADE_SYMBOL,))
        return cur.fetchall()

def get_total_spent(conn):
    with conn.cursor() as cur:
        cur.execute(
            "SELECT COALESCE(SUM(t.quote_qty),0) FROM trades t JOIN auto_positions ap ON ap.trade_id=t.order_id WHERE t.side='BUY' AND t.symbol=%s",
            (TRADE_SYMBOL,)
        )
        return float(cur.fetchone()[0])

# ✅ NOVO: busca o stepSize real do par
def get_step_size(symbol: str) -> float:
    client = get_binance_client()
    info = client.get_symbol_info(symbol)
    lot_size = next(f for f in info['filters'] if f['filterType'] == 'LOT_SIZE')
    return float(lot_size['stepSize'])

# ✅ NOVO: ajusta usando stepSize real
def adjust_to_step_size(qty: float, symbol: str) -> float:
    step_size = get_step_size(symbol)
    qty_dec = Decimal(str(qty))
    step_dec = Decimal(str(step_size))
    adjusted_qty = (qty_dec // step_dec) * step_dec
    return float(adjusted_qty)

def sell_position(conn, symbol: str, qty: float, trade_order_id: int):
    qty = adjust_to_step_size(qty, symbol)
    if qty <= 0:
        print(f"[AUTO-SELL] Skipping sell: qty after adjustment is {qty}")
        return None
    payload = {'symbol': symbol, 'side': 'SELL', 'quantity': qty}
    try:
        resp = requests.post(f"{API_URL}/order", json=payload)
        resp.raise_for_status()
        result = resp.json()
        print(f"[AUTO-SELL] Sold {qty} {symbol}: {result}")
        with conn.cursor() as cur:
            cur.execute(
                "UPDATE auto_positions SET sell_date = NOW() WHERE trade_id = %s",
                (trade_order_id,)
            )
        return result
    except Exception as e:
        print(f"[AUTO-SELL] Error selling {symbol}: {e}")
        return None

def buy_position(conn, symbol: str, qty: float):
    qty = adjust_to_step_size(qty, symbol)
    if qty <= 0:
        print(f"[AUTO-BUY] Skipping buy: qty after adjustment is {qty}")
        return None
    payload = {'symbol': symbol, 'side': 'BUY', 'quantity': qty}
    try:
        resp = requests.post(f"{API_URL}/order", json=payload)
        resp.raise_for_status()
        result = resp.json()
        print(f"[AUTO-BUY] Bought {qty} {symbol}: {result}")
        order = result.get('order', {})
        trade_id = order.get('orderId')
        if trade_id:
            with conn.cursor() as cur:
                cur.execute(
                    "INSERT INTO auto_positions(trade_id, purchase_date) VALUES(%s, NOW()) ON CONFLICT DO NOTHING",
                    (trade_id,)
                )
        return result
    except Exception as e:
        print(f"[AUTO-BUY] Error buying {symbol}: {str(e)}")
        return None

def get_current_price(symbol: str) -> float:
    client = get_binance_client()
    ticker = client.get_symbol_ticker(symbol=symbol)
    return float(ticker['price'])

def main():
    conn = get_db_connection()
    print(f"Service starting for {TRADE_SYMBOL}: cap quote {PURCHASE_LIMIT_QUOTE}, sell @+{SELL_THRESHOLD_PCT}% every {POLL_INTERVAL}s")

    while True:
        try:
            open_buys = get_open_buys(conn)

            if open_buys:
                for order_id, symbol, qty, buy_price in open_buys:
                    target_price = buy_price * (1 + SELL_THRESHOLD_PCT / 100)
                    current_price = get_current_price(symbol)
                    if current_price >= target_price:
                        print(f"[AUTO-SELL] Trade {order_id} target {target_price:.2f}, current {current_price:.2f}")
                        sell_position(conn, symbol, qty, order_id)
            else:
                total_spent = get_total_spent(conn)
                remaining = PURCHASE_LIMIT_QUOTE - total_spent
                if remaining > 0:
                    price = get_current_price(TRADE_SYMBOL)
                    qty_to_buy = remaining / price
                    print(f"[AUTO-BUY] No open positions, buying {qty_to_buy:.6f} {TRADE_SYMBOL} (~{remaining})")
                    buy_position(conn, TRADE_SYMBOL, qty_to_buy)
        except Exception as e:
            print(f"[AUTO-SELL] Loop error: {e}")

        time.sleep(POLL_INTERVAL)

if __name__ == '__main__':
    main()
