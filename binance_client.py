import os
from dotenv import load_dotenv
from binance.client import Client
from binance.enums import SIDE_BUY, SIDE_SELL, ORDER_TYPE_MARKET

load_dotenv()

USE_TESTNET = os.getenv("BINANCE_TESTNET", "false").lower() == "true"

if USE_TESTNET:
    BINANCE_API_KEY = os.getenv("BINANCE_TESTNET_API_KEY")
    BINANCE_API_SECRET = os.getenv("BINANCE_TESTNET_API_SECRET")
else:
    BINANCE_API_KEY = os.getenv("BINANCE_API_KEY")
    BINANCE_API_SECRET = os.getenv("BINANCE_API_SECRET")

def get_binance_client():
    if not BINANCE_API_KEY or not BINANCE_API_SECRET:
        raise Exception("Binance API Key or Secret not set in environment variables.")

    client = Client(api_key=BINANCE_API_KEY, api_secret=BINANCE_API_SECRET)

    if USE_TESTNET:
        print("Using Binance Testnet")
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

from decimal import Decimal, ROUND_DOWN
from binance.enums import SIDE_BUY, SIDE_SELL, ORDER_TYPE_MARKET

def place_market_order(symbol: str,
                       side: str,
                       quantity: float = None,
                       quoteOrderQty: float = None):
    """
    - Either quantity (base asset) or quoteOrderQty (quote asset spend) must be provided.
    - Returns on success: {"status":"success","order": order,...}
    - On LOT_SIZE error: returns {"status":"error", "message":..., "minQty":..., "stepSize":...}
    - On other error: {"status":"error","message":...}
    """
    client = get_binance_client()
    # build params dict
    params = {
        "symbol": symbol,
        "side": SIDE_BUY if side.upper()=="BUY" else SIDE_SELL,
        "type": ORDER_TYPE_MARKET
    }
    if quoteOrderQty is not None:
        # format as plain decimal
        params["quoteOrderQty"] = format(Decimal(str(quoteOrderQty)),'f')
    else:
        params["quantity"] = format(Decimal(str(quantity)), 'f')

    try:
        order = client.create_order(**params)
        return {"status":"success", "order": order, "onTestnet": USE_TESTNET}
    except Exception as e:
        err = str(e)
        # Catch LOT_SIZE filter failures and return minQty/stepSize
        if "Filter failure: LOT_SIZE" in err:
            info = client.get_symbol_info(symbol)
            lot = next(f for f in info['filters'] if f['filterType']=="LOT_SIZE")
            return {
                "status":"error",
                "message": (
                    f"Quantity too small or invalid for {symbol}. "
                    f"Minimum is {lot['minQty']} and increments of {lot['stepSize']}."
                ),
                "minQty": float(lot['minQty']),
                "stepSize": float(lot['stepSize'])
            }
        return {"status":"error", "message": err}
    