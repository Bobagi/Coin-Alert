import os, sys
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import time
import requests
import psycopg2
from dotenv import load_dotenv
from decimal import Decimal
from datetime import datetime, date, timezone
from binance_client import get_binance_client

# Load environment variables
dotenv_path = os.path.join(os.path.dirname(__file__), '..', '.env')
load_dotenv(dotenv_path)

# Configuration
DB_CONFIG = {
    'host': os.getenv('DB_HOST'),
    'dbname': os.getenv('DB_NAME'),
    'user': os.getenv('DB_USER'),
    'password': os.getenv('DB_PASSWORD'),
    'port': os.getenv('DB_PORT', 5432)
}
API_URL               = os.getenv('API_URL', 'http://api:5000')
TRADE_SYMBOL          = os.getenv('TRADE_SYMBOL', 'BTCBRL')
SELL_THRESHOLD_PCT    = Decimal(os.getenv('SELL_THRESHOLD_PCT', '1.0'))
PURCHASE_LIMIT_QUOTE  = Decimal(os.getenv('PURCHASE_LIMIT_QUOTE', '100.0'))
BUY_INCREMENT_QUOTE   = Decimal(os.getenv('BUY_INCREMENT_QUOTE', '10.0'))
BUY_DELAY_HOURS       = int(os.getenv('BUY_DELAY_HOURS', '1'))
thresh_ts_str         = os.getenv('SELL_AFTER_TIMESTAMP', '2025-04-29T23:23:16')
SELL_AFTER_TIMESTAMP  = datetime.fromisoformat(thresh_ts_str).replace(tzinfo=timezone.utc)
POLL_INTERVAL         = int(os.getenv('POLL_INTERVAL_SECONDS', 60))

def get_db_connection():
    conn = psycopg2.connect(**DB_CONFIG)
    conn.autocommit = True
    return conn

def get_pending_positions(conn):
    with conn.cursor() as cur:
        cur.execute(
            "SELECT ap.trade_id, ap.purchase_date, t.qty::float, t.price::float "
            "FROM auto_positions ap "
            "JOIN trades t ON t.order_id = ap.trade_id "
            "WHERE ap.sell_trade_id IS NULL "
            "  AND ap.purchase_date >= %s;",
            (SELL_AFTER_TIMESTAMP,)
        )
        return cur.fetchall()

def get_total_spent_today(conn):
    with conn.cursor() as cur:
        cur.execute(
            "SELECT COALESCE(SUM(t.quote_qty),0) "
            "FROM trades t JOIN auto_positions ap ON ap.trade_id=t.order_id "
            "WHERE t.side='BUY' AND t.symbol=%s AND ap.purchase_date::date = %s",
            (TRADE_SYMBOL, date.today())
        )
        return Decimal(str(cur.fetchone()[0]))

def get_symbol_filters(symbol: str):
    info = get_binance_client().get_symbol_info(symbol)
    lot        = next(f for f in info['filters'] if f['filterType']=='LOT_SIZE')
    notional   = next(f for f in info['filters'] if f['filterType']=='NOTIONAL')
    price_filt = next(f for f in info['filters'] if f['filterType']=='PRICE_FILTER')
    return (
        Decimal(lot['stepSize']),
        Decimal(notional['minNotional']),
        Decimal(price_filt['tickSize']),
    )

def adjust_to_step_size(qty: Decimal, step: Decimal) -> Decimal:
    return (qty // step) * step

def adjust_to_tick_size(price: Decimal, tick: Decimal) -> Decimal:
    return (price // tick) * tick

def get_current_price(symbol: str) -> Decimal:
    ticker = get_binance_client().get_symbol_ticker(symbol=symbol)
    return Decimal(ticker['price'])

def send_order(params: dict):
    r = requests.post(f"{API_URL}/order", json=params)
    try:
        return r.json()
    except:
        return {'status':'error','message':r.text}

def send_limit_order(params: dict):
    r = requests.post(f"{API_URL}/limit-order", json=params)
    try:
        return r.json()
    except:
        return {'status':'error','message':r.text}

def process_sells(conn):
    pending = get_pending_positions(conn)
    if not pending:
        return

    total_qty = sum(Decimal(str(qty)) for (_, _, qty, _) in pending)
    step, min_notional, tick = get_symbol_filters(TRADE_SYMBOL)

    qty_adj = adjust_to_step_size(total_qty, step)
    if qty_adj <= 0:
        print(f"[AUTO-SELL] Adjusted qty {qty_adj} below step {step}")
        return

    current_price = get_current_price(TRADE_SYMBOL)
    raw_limit_price = current_price * (1 + SELL_THRESHOLD_PCT / Decimal(100))
    limit_price     = adjust_to_tick_size(raw_limit_price, tick)

    notional = qty_adj * limit_price
    if notional < min_notional:
        print(f"[AUTO-SELL] Notional {notional} below minimum {min_notional}")
        return

    params = {
        'symbol':   TRADE_SYMBOL,
        'side':     'SELL',
        'quantity': format(qty_adj, 'f'),
        'price':    format(limit_price, 'f'),
    }
    result = send_limit_order(params)
    print(f"[AUTO-SELL] LIMIT sell {qty_adj}@{limit_price}: {result}")

    if result.get('status') == 'success':
        sell_id   = result['order']['orderId']
        trade_ids = [t for (t, _, _, _) in pending]
        with conn.cursor() as cur:
            cur.execute(
                "UPDATE auto_positions SET sell_trade_id=%s, sell_date=NOW() WHERE trade_id=ANY(%s)",
                (sell_id, trade_ids)
            )

def process_buys(conn, last_buy_time):
    # unchanged...
    spent = get_total_spent_today(conn)
    remaining = PURCHASE_LIMIT_QUOTE - spent
    now = datetime.now(timezone.utc)
    if last_buy_time and (now - last_buy_time).total_seconds() < BUY_DELAY_HOURS*3600:
        return last_buy_time
    if remaining <= 0:
        return last_buy_time
    increment = BUY_INCREMENT_QUOTE if remaining >= BUY_INCREMENT_QUOTE else remaining
    params = {'symbol': TRADE_SYMBOL, 'side': 'BUY', 'quoteOrderQty': format(increment, 'f')}
    result = send_order(params)
    print(f"[AUTO-BUY] Buy {increment} {TRADE_SYMBOL}: {result}")
    if result.get('status') == 'success':
        trade_id = result['order']['orderId']
        with conn.cursor() as cur:
            cur.execute(
                "INSERT INTO auto_positions(trade_id, purchase_date) VALUES(%s, NOW()) ON CONFLICT DO NOTHING;",
                (trade_id,)
            )
        return now
    return last_buy_time

def main():
    conn = get_db_connection()
    last_buy_time = None
    print(f"Service start: cap R${PURCHASE_LIMIT_QUOTE}, incr R${BUY_INCREMENT_QUOTE}, delay {BUY_DELAY_HOURS}h")
    while True:
        try:
            process_sells(conn)
            #last_buy_time = process_buys(conn, last_buy_time)
        except Exception as e:
            print(f"[AUTO-SELL] Loop error: {e}")
        time.sleep(POLL_INTERVAL)

if __name__=='__main__':
    main()
