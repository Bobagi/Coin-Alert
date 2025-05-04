import sys, os, time, requests
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from decimal import Decimal
from datetime import datetime, timezone
from collections import defaultdict

from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from logger_config import setup_logger
from models import DailyPurchaseConfig, UserCredentials
from binance_client import place_market_order

logger = setup_logger("daily-buy")

API_URL = os.getenv("API_URL", "http://api:5020")
DAILY_SPEND = Decimal(os.getenv("DAILY_SPEND_BRL", "10"))
DIP_HOUR_UTC = int(os.getenv("DIP_HOUR_UTC", "4"))
BINANCE_REST = "https://api.binance.com"

DATABASE_URL = os.getenv("DATABASE_URL")
engine = create_engine(DATABASE_URL)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

def fetch_price(symbol):
    resp = requests.get(f"{BINANCE_REST}/api/v3/ticker/price", params={"symbol": symbol})
    resp.raise_for_status()
    return Decimal(resp.json()["price"])

def fetch_step_size(symbol):
    info = requests.get(f"{BINANCE_REST}/api/v3/exchangeInfo", params={"symbol": symbol})
    info.raise_for_status()
    filt = next(f for f in info.json()["symbols"][0]["filters"] if f["filterType"] == "LOT_SIZE")
    return Decimal(filt["stepSize"])

def compute_qty(price: Decimal, spend: Decimal, step: Decimal) -> float:
    raw = spend / price
    units = (raw // step)
    if raw % step != 0:
        units += 1
    qty = units * step
    return float(qty)

def place_order(symbol: str, qty: float, user_id: int):
    payload = {
        "symbol": symbol,
        "side": "BUY",
        "quantity": qty,
        "operation_type": "DAILY",
        "userId": user_id
    }
    r = requests.post(f"{API_URL}/order", json=payload)
    return r.status_code, r.text

def main():
    last_date = None
    logger.info("Starting daily buy script...")

    while True:
        now = datetime.now(timezone.utc)
        if now.hour == DIP_HOUR_UTC and now.date() != last_date:
            logger.info("It's time to buy!")
            last_date = now.date()
            db_session = SessionLocal()
            try:
                configs = db_session.query(DailyPurchaseConfig).order_by(DailyPurchaseConfig.user_id).all()
                user_configs = defaultdict(list)
                for config in configs:
                    user_configs[config.user_id].append(config)

                for user_id, configs in user_configs.items():
                    for config in configs:
                        symbol = config.crypto_symbol
                        amount_in_currency = Decimal(config.amount_brl)
                        try:
                            price = fetch_price(symbol)
                            step = fetch_step_size(symbol)
                            qty = compute_qty(price, amount_in_currency, step)
                            code, body = place_order(symbol, qty, user_id)
                            logger.info(f"{now.isoformat()} | DAILY BUY {qty} {symbol} (~R${amount_in_currency}) → {code}: {body}")
                            
                        except Exception as e:
                            logger.error(f"{now.isoformat()} | ERROR executing daily buy for user {user_id}, symbol {symbol}: {e}")
            finally:
                db_session.close()
        time.sleep(10)

if __name__ == "__main__":
    main()
