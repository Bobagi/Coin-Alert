import sys, os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import time
import requests
import psycopg2
from dotenv import load_dotenv
from decimal import Decimal, ROUND_CEILING
from datetime import datetime, timezone
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from binance_client import get_binance_client
from logger_config import setup_logger
from models import AutoBuyQuota  # adjust import path if needed

logger = setup_logger("auto-sell")

# load env
dotenv_path = os.path.join(os.path.dirname(__file__), '..', '.env')
load_dotenv(dotenv_path)

# DB config from env
DB_CONFIG = {
    'host':     os.getenv('DB_HOST'),
    'dbname':   os.getenv('DB_NAME'),
    'user':     os.getenv('DB_USER'),
    'password': os.getenv('DB_PASSWORD'),
    'port':     os.getenv('DB_PORT', 5432)
}

API_URL            = os.getenv('API_URL', 'http://api:5020')
SELL_THRESHOLD_PCT = Decimal(os.getenv('SELL_THRESHOLD_PCT', '1.0'))
BUY_DELAY_HOURS    = int(os.getenv('BUY_DELAY_HOURS', '1'))
thresh_ts_str      = os.getenv('SELL_AFTER_TIMESTAMP', '2025-04-29T23:23:16')
SELL_AFTER_TIMESTAMP = datetime.fromisoformat(thresh_ts_str).replace(tzinfo=timezone.utc)
POLL_INTERVAL      = int(os.getenv('POLL_INTERVAL_SECONDS', 60))

# SQLAlchemy session for quotas
engine = create_engine(
    f"postgresql://{DB_CONFIG['user']}:{DB_CONFIG['password']}@"
    f"{DB_CONFIG['host']}:{DB_CONFIG['port']}/{DB_CONFIG['dbname']}"
)
SessionLocal = sessionmaker(bind=engine)

def get_db_connection():
    conn = psycopg2.connect(**DB_CONFIG)
    conn.autocommit = True
    return conn

def get_symbol_filters(symbol: str, user_id: int):
    client = get_binance_client(user_id)
    info      = client.get_symbol_info(symbol)
    lot       = next(f for f in info['filters'] if f['filterType']=='LOT_SIZE')
    notional  = next(f for f in info['filters'] if f['filterType']=='NOTIONAL')
    price_flt = next(f for f in info['filters'] if f['filterType']=='PRICE_FILTER')
    return (
        Decimal(lot['stepSize']),
        Decimal(notional['minNotional']),
        Decimal(price_flt['tickSize']),
    )

def adjust_to_step_size(qty: Decimal, step: Decimal) -> Decimal:
    return (qty // step) * step

def adjust_to_tick_size(price: Decimal, tick: Decimal) -> Decimal:
    return (price // tick) * tick

def ceil_to_step(qty: Decimal, step: Decimal) -> Decimal:
    units = (qty / step).quantize(0, rounding=ROUND_CEILING)
    return units * step

def get_current_price(symbol: str, user_id: int) -> Decimal:
    ticker = get_binance_client(user_id).get_symbol_ticker(symbol=symbol)
    return Decimal(ticker['price'])

def send_order(params: dict):
    r = requests.post(f"{API_URL}/order", json=params)
    try:
        return r.json()
    except:
        return {'status':'error', 'message': r.text}

def send_limit_order(params: dict):
    r = requests.post(f"{API_URL}/limit-order", json=params)
    try:
        return r.json()
    except:
        return {'status':'error', 'message': r.text}

def get_pending_positions(conn, user_id, symbol):
    with conn.cursor() as cur:
        cur.execute("""
            SELECT ap.trade_id, ap.purchase_date, t.qty::float, t.price::float
              FROM auto_positions ap
              JOIN trades t ON t.order_id = ap.trade_id
             WHERE ap.sell_trade_id IS NULL
               AND ap.purchase_date >= %s
               AND ap.user_id = %s
               AND t.symbol = %s;
        """, (SELL_AFTER_TIMESTAMP, user_id, symbol))
        return cur.fetchall()

def get_current_exposure(conn, user_id, symbol):
    with conn.cursor() as cur:
        cur.execute("""
            SELECT COALESCE(SUM(t.quote_qty),0)
              FROM trades t
              JOIN auto_positions ap ON ap.trade_id = t.order_id
             WHERE t.side='BUY'
               AND t.symbol=%s
               AND ap.sell_date IS NULL
               AND ap.user_id = %s;
        """, (symbol, user_id))
        return Decimal(str(cur.fetchone()[0]))

def process_fills(conn, user_id, symbol):
    logger.info(f"user {user_id} — check filled limit orders for {symbol}")
    with conn.cursor() as cur:
        cur.execute("""
            SELECT ap.trade_id, ap.sell_trade_id
              FROM auto_positions ap
             WHERE ap.sell_trade_id IS NOT NULL
               AND ap.sell_date IS NULL
               AND ap.user_id = %s;
        """, (user_id,))
        rows = cur.fetchall()

    client = get_binance_client(user_id)
    for buy_id, sell_id in rows:
        try:
            order = client.get_order(symbol=symbol, orderId=sell_id)
        except Exception as e:
            logger.error(f"user {user_id} — error fetching order {sell_id}: {e}")
            continue
        if order['status'] == 'FILLED':
            exec_qty   = Decimal(order['executedQty'])
            exec_quote = Decimal(order['cummulativeQuoteQty'])
            with conn.cursor() as cur:
                cur.execute("UPDATE auto_positions SET sell_date=NOW() WHERE trade_id=%s;", (buy_id,))
                cur.execute(
                    "UPDATE trades SET status=%s, qty=%s, quote_qty=%s WHERE order_id=%s;",
                    ('FILLED', exec_qty, exec_quote, sell_id)
                )
            locked = get_current_exposure(conn, user_id, symbol)
            logger.info(f"user {user_id} — sold {symbol}: released R${exec_quote:.8f}, locked now R${locked:.8f}")

def process_sells(conn, user_id, symbol):
    pending = get_pending_positions(conn, user_id, symbol)
    if not pending:
        return

    total_qty  = sum(Decimal(str(q)) for (_, _, q, _) in pending)
    total_cost = sum(Decimal(str(q)) * Decimal(str(p)) for (_, _, q, p) in pending)
    avg_price  = total_cost / total_qty

    step, min_notional, tick = get_symbol_filters(symbol, user_id)
    qty_adj = adjust_to_step_size(total_qty, step)
    if qty_adj <= 0:
        logger.warning(f"user {user_id} — adjusted qty {qty_adj} below step {step}")
        return

    raw_limit_price = avg_price * (1 + SELL_THRESHOLD_PCT / Decimal(100))
    limit_price     = adjust_to_tick_size(raw_limit_price, tick)
    notional        = qty_adj * limit_price

    if notional < min_notional:
        logger.warning(f"user {user_id} — grouped notional {notional} below minNotional {min_notional}")
        return

    params = {
        'symbol':         symbol,
        'side':           'SELL',
        'quantity':       format(qty_adj, 'f'),
        'price':          format(limit_price, 'f'),
        'operation_type': 'AUTOS',
        'userId':         user_id
    }
    result = send_limit_order(params)
    logger.info(f"user {user_id} — LIMIT sell {symbol} {qty_adj}@{limit_price}: {result}")
    if result.get('status') == 'success':
        sell_id   = result['order']['orderId']
        trade_ids = [t for (t, _, _, _) in pending]
        with conn.cursor() as cur:
            cur.execute(
                "UPDATE auto_positions SET sell_trade_id=%s WHERE trade_id=ANY(%s) AND user_id=%s;",
                (sell_id, trade_ids, user_id)
            )

def process_buys(conn, session, quota, last_buy_time):
    user_id = quota.user_id
    symbol  = quota.crypto_symbol

    locked    = get_current_exposure(conn, user_id, symbol)
    remaining = Decimal(quota.quota_limit_brl) - Decimal(quota.quota_used_brl)
    logger.info(f"user {user_id} — {symbol}: locked R${locked:.8f}, quota left R${remaining:.8f}")

    now = datetime.now(timezone.utc)
    if last_buy_time and (now - last_buy_time).total_seconds() < BUY_DELAY_HOURS * 3600:
        return last_buy_time
    if remaining <= 0:
        logger.info(f"user {user_id} — {symbol}: quota exhausted")
        return last_buy_time

    increment = remaining
    params = {
        'symbol':         symbol,
        'side':           'BUY',
        'quoteOrderQty':  format(increment, 'f'),
        'operation_type': 'AUTOB',
        'userId':         user_id
    }
    result = send_order(params)
    logger.info(f"user {user_id} — BUY {symbol} R${increment}: {result}")

    if result.get('status') == 'success':
        trade_id = result['order']['orderId']
        with conn.cursor() as cur:
            cur.execute(
                "INSERT INTO auto_positions(trade_id, purchase_date, user_id) VALUES(%s, NOW(), %s) ON CONFLICT DO NOTHING;",
                (trade_id, user_id)
            )
        session.query(AutoBuyQuota).filter_by(id=quota.id).update({
            'quota_used_brl': AutoBuyQuota.quota_used_brl + increment
        })
        session.commit()
        return now

    return last_buy_time

def main():
    session = SessionLocal()
    conn = get_db_connection()
    last_buy_times = {}

    logger.info("Service start")
    while True:
        quotas = session.query(AutoBuyQuota).order_by(AutoBuyQuota.user_id).all()
        for quota in quotas:
            key = (quota.user_id, quota.crypto_symbol)
            last_buy = last_buy_times.get(key)

            process_fills(conn, quota.user_id, quota.crypto_symbol)
            process_sells(conn, quota.user_id, quota.crypto_symbol)
            new_time = process_buys(conn, session, quota, last_buy)
            last_buy_times[key] = new_time

        time.sleep(POLL_INTERVAL)

if __name__ == '__main__':
    main()
