import requests

def get_current_price(symbol):
    url = f"https://api.coingecko.com/api/v3/simple/price?ids={symbol}&vs_currencies=usd"
    response = requests.get(url)
    data = response.json()
    return data[symbol]['usd']

def main():
    crypto_symbol = 'bitcoin'  # Replace with the symbol of the cryptocurrency you want to get the price for

    current_price = get_current_price(crypto_symbol)
    print(f"The current price of {crypto_symbol.upper()} is ${current_price}")

if __name__ == "__main__":
    main()
