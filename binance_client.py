import os
from dotenv import load_dotenv
from binance.client import Client
from binance.enums import SIDE_BUY, SIDE_SELL, ORDER_TYPE_MARKET, ORDER_TYPE_LIMIT, TIME_IN_FORCE_GTC
from decimal import Decimal

load_dotenv()

USE_TESTNET = os.getenv("BINANCE_TESTNET", "false").lower() == "true"
if USE_TESTNET:
    API_KEY = os.getenv("BINANCE_TESTNET_API_KEY")
    API_SECRET = os.getenv("BINANCE_TESTNET_API_SECRET")
else:
    API_KEY = os.getenv("BINANCE_API_KEY")
    API_SECRET = os.getenv("BINANCE_API_SECRET")


def get_binance_client():
    client = Client(api_key=API_KEY, api_secret=API_SECRET)
    if USE_TESTNET:
        client.API_URL = 'https://testnet.binance.vision/api'
    return client

def get_asset_balance(asset: str = "BTC"):
    client = get_binance_client()
    try:
        info = client.get_account()
        bal = next((b for b in info["balances"] if b["asset"] == asset.upper()), None)
        if bal is None:
            return {"status": "error", "message": f"Asset {asset} not found"}
        return {"status": "success", f"{asset.upper()}_balance": bal}
    except Exception as e:
        return {"status": "error", "message": str(e)}
    

def place_market_order(symbol: str, side: str, quantity: float = None, quoteOrderQty: float = None):
    client = get_binance_client()
    params = {"symbol": symbol, "side": SIDE_BUY if side.upper()=="BUY" else SIDE_SELL, "type": ORDER_TYPE_MARKET}
    if quoteOrderQty is not None:
        params["quoteOrderQty"] = format(Decimal(str(quoteOrderQty)), 'f')
    else:
        params["quantity"] = format(Decimal(str(quantity)), 'f')

    try:
        order = client.create_order(**params)
        return {"status":"success", "order": order, "onTestnet": USE_TESTNET}
    except Exception as e:
        return {"status":"error", "message": str(e)}


def place_limit_order(symbol: str, side: str, quantity: float, price: float):
    client = get_binance_client()
    qty_str   = format(Decimal(str(quantity)), 'f')
    price_str = format(Decimal(str(price)),    'f')
    try:
        order = client.create_order(
            symbol=symbol,
            side=SIDE_SELL if side.upper()=="SELL" else SIDE_BUY,
            type=ORDER_TYPE_LIMIT,
            timeInForce=TIME_IN_FORCE_GTC,
            quantity=qty_str,
            price=price_str
        )
        return {"status":"success", "order": order, "onTestnet": USE_TESTNET}
    except Exception as e:
        return {"status":"error", "message": str(e)}
    