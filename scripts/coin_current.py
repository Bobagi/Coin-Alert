import sys, os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import requests
from logger_config import setup_logger

logger = setup_logger("coin-current")

def get_current_price(symbol):
    url = f"https://api.coingecko.com/api/v3/simple/price?ids={symbol}&vs_currencies=usd"
    response = requests.get(url)
    data = response.json()
    return data[symbol]['usd']

def main():
    crypto_symbol = 'bitcoin'
    current_price = get_current_price(crypto_symbol)
    logger.info(f"The current price of {crypto_symbol.upper()} is ${current_price}")

if __name__ == "__main__":
    main()