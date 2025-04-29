#!/usr/bin/env python3
import os
import time
import requests
from decimal import Decimal
from datetime import datetime, timezone

# ← configure these via your .env if you like
API_URL        = os.getenv("API_URL", "http://api:5000")
SYMBOL         = os.getenv("TRADE_SYMBOL", "BTCBRL")
DAILY_SPEND    = Decimal(os.getenv("DAILY_SPEND_BRL", "10"))
DIP_HOUR_UTC   = int(os.getenv("DIP_HOUR_UTC", "4"))  # 04:00 UTC

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
    # round UP to the next multiple of stepSize
    units = (raw // step)
    if raw % step != 0:
        units += 1
    qty = units * step
    return float(qty)

def place_order(qty: float):
    payload = {"symbol": SYMBOL, "side": "BUY", "quantity": qty}
    r = requests.post(f"{API_URL}/order", json=payload)
    return r.status_code, r.text

def main():
    last_date = None

    while True:
        now = datetime.now(timezone.utc)
        if now.hour == DIP_HOUR_UTC and now.date() != last_date:
            print(f"Its time to buy!...")
            last_date = now.date()
            step = fetch_step_size()
            try:
                price = fetch_price()
                qty   = compute_qty(price, DAILY_SPEND, step)
                code, body = place_order(qty)
                print(f"{now.isoformat()} | BUY {qty} {SYMBOL} (~R${DAILY_SPEND}) → {code}: {body}")
            except Exception as e:
                print(f"{now.isoformat()} | ERROR executing daily buy: {e}")
        time.sleep(10)

if __name__ == "__main__":
    print(f"Starting daily buy script...")
    main()
