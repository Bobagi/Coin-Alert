import sys, os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import requests
import smtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from dotenv import load_dotenv
from logger_config import setup_logger

load_dotenv()
logger = setup_logger("coin-monitor")

def get_crypto_data(symbol):
    url = f"https://api.coingecko.com/api/v3/simple/price?ids={symbol}&vs_currencies=usd"
    response = requests.get(url)
    data = response.json()
    return data[symbol]['usd']

def get_all_cryptos():
    url = "https://api.coingecko.com/api/v3/coins/list"
    response = requests.get(url)
    data = response.json()
    return [crypto['symbol'] for crypto in data]

def send_email_alert(to_email, subject, body):
    from_email = os.getenv("EMAIL")
    password = os.getenv("PASSWORD")

    msg = MIMEMultipart()
    msg['From'] = from_email
    msg['To'] = to_email
    msg['Subject'] = subject
    msg.attach(MIMEText(body, 'plain'))

    server = smtplib.SMTP('smtp.gmail.com', 587)
    server.starttls()
    server.login(from_email, password)
    server.sendmail(from_email, to_email, msg.as_string())
    server.quit()
    logger.info(f"Sent email alert to {to_email} with subject '{subject}'")

def main():
    crypto_symbol = 'bitcoin'
    threshold_price = 50000

    all_cryptos = get_all_cryptos()
    logger.info(f"Loaded {len(all_cryptos)} cryptocurrencies from CoinGecko")

    current_price = get_crypto_data(crypto_symbol)
    if current_price > threshold_price:
        logger.info("Current price exceeds threshold, sending email...")
        send_email_alert(os.getenv("DESTINY"), 'Crypto Alert', f'The price of {crypto_symbol.upper()} has crossed ${threshold_price}.')

if __name__ == "__main__":
    main()
