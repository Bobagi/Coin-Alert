import sys, os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import time
import requests
from decimal import Decimal
from datetime import datetime, timezone
from logger_config import setup_logger

logger = setup_logger("daily-buy")

API_URL        = os.getenv("API_URL", "http://api:5020")
SYMBOL         = os.getenv("TRADE_SYMBOL", "BTCBRL")
DAILY_SPEND    = Decimal(os.getenv("DAILY_SPEND_BRL", "10"))
DIP_HOUR_UTC   = int(os.getenv("DIP_HOUR_UTC", "4"))

BINANCE_REST   = "https://api.binance.com"

def fetch_price():
    resp = requests.get(f"{BINANCE_REST}/api/v3/ticker/price", params={"symbol": SYMBOL})
    resp.raise_for_status()
    return Decimal(resp.json()["price"])

def fetch_step_size():
    info = requests.get(f"{BINANCE_REST}/api/v3/exchangeInfo", params={"symbol": SYMBOL})
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

def place_order(qty: float):
    payload = {"symbol": SYMBOL, "side": "BUY", "quantity": qty, "operation_type": "DAILY"}
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
            step = fetch_step_size()
            try:
                price = fetch_price()
                qty   = compute_qty(price, DAILY_SPEND, step)
                code, body = place_order(qty)
                logger.info(f"{now.isoformat()} | DAILY BUY {qty} {SYMBOL} (~R${DAILY_SPEND}) → {code}: {body}")
            except Exception as e:
                logger.error(f"{now.isoformat()} | ERROR executing daily buy: {e}")
        time.sleep(10)

if __name__ == "__main__":
    main()
