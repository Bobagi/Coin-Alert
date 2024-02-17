import os
import requests
import smtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from dotenv import load_dotenv

load_dotenv()

# Function to fetch cryptocurrency data
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

# Function to send email alert
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
    text = msg.as_string()
    server.sendmail(from_email, to_email, text)
    server.quit()

# Main function
def main():
    # Replace 'btc' with the symbol of the cryptocurrency you want to monitor
    crypto_symbol = 'bitcoin'
    threshold_price = 50000  # Update with your desired threshold price

    all_cryptos = get_all_cryptos()
    print(f"All possible cryptocurrencies: {all_cryptos}")

    current_price = get_crypto_data(crypto_symbol)
    if current_price > threshold_price:
        print(f"Current price bigger than threshold, sending email...")
        send_email_alert(os.getenv("DESTINY"), 'Crypto Alert', f'The price of {crypto_symbol.upper()} has crossed ${threshold_price}.')

if __name__ == "__main__":
    main()
