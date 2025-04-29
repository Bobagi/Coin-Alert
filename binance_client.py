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

def place_market_order(symbol: str, side: str, quantity: float):
    """
    Place a market order with quantity formatted as plain decimal string.
    """
    client = get_binance_client()
    # Convert to Decimal then to string to avoid scientific notation
    qty_dec = Decimal(str(quantity))
    qty_str = format(qty_dec, 'f')  # e.g. "0.00002"
    try:
        order = client.create_order(
            symbol=symbol,
            side=SIDE_BUY if side.upper() == "BUY" else SIDE_SELL,
            type=ORDER_TYPE_MARKET,
            quantity=qty_str
        )
        return {"status": "success", "order": order, "onTestnet": USE_TESTNET}
    except Exception as e:
        return {"status": "error", "message": str(e)}

