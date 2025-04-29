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

def test_binance_connection():
    client = get_binance_client()
    try:
        info = client.get_account()
        btc = next((b for b in info['balances'] if b['asset'] == 'BTC'), None)
        return {"status":"success","BTC_balance":btc}
    except Exception as e:
        return {"status":"error","message":str(e)}

def place_market_order(symbol: str, side: str, quantity: float):
    """
    symbol: ex. "BTCUSDT"
    side: "BUY" ou "SELL"
    quantity: quantidade na unidade base (ex: 0.001 para BTC)
    """
    client = get_binance_client()
    try:
        order = client.create_order(
            symbol=symbol,
            side=SIDE_BUY   if side.upper()=="BUY"  else SIDE_SELL,
            type=ORDER_TYPE_MARKET,
            quantity=quantity,
            onTestnet=USE_TESTNET
        )
        return {"status":"success", "order": order}
    except Exception as e:
        return {"status":"error",   "message": str(e)}
