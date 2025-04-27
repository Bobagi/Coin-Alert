import os
from dotenv import load_dotenv
from binance.client import Client

load_dotenv()

BINANCE_API_KEY = os.getenv("BINANCE_API_KEY")
BINANCE_API_SECRET = os.getenv("BINANCE_API_SECRET")

def get_binance_client():
    if not BINANCE_API_KEY or not BINANCE_API_SECRET:
        raise Exception("Binance API Key or Secret not set in environment variables.")
    client = Client(api_key=BINANCE_API_KEY, api_secret=BINANCE_API_SECRET)
    return client

def test_binance_connection():
    client = get_binance_client()
    try:
        account_info = client.get_account()
        balances = account_info['balances']
        btc_balance = next((balance for balance in balances if balance['asset'] == 'BTC'), None)
        return {
            "status": "success",
            "BTC_balance": btc_balance
        }
    except Exception as e:
        return {
            "status": "error",
            "message": str(e)
        }
