import os
import smtplib
import time
import requests
from colorama import Fore, Style
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from dotenv import load_dotenv

load_dotenv()

def send_email(subject, body, to_email):
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

def clear_alerts():
    print(f"Cleaning alerts...")
    url = "https://bobagi.net/api/cryptoAlert/clearAlerts"
    try:
        response = requests.post(url)
        response.raise_for_status()
        print("Alerts cleared successfully!")
    except requests.exceptions.RequestException as e:
        print(f"Error while clearing alerts: {e}")

def get_cryptos():
    print(f"Getting cryptos...")
    try:
        url = "https://bobagi.net/api/cryptoAlert/getCryptos"
        response = requests.get(url)
        response.raise_for_status()
        return response.json()
    except requests.exceptions.RequestException as e:
        print(f"Error while fetching cryptocurrencies: {e}")
        return []
    
def get_crypto_value(crypto):
    try:
        url = f"https://api.coingecko.com/api/v3/simple/price?ids={crypto}&vs_currencies=usd"
        response = requests.get(url)
        response.raise_for_status()
        return response.json()[crypto]["usd"]
    except requests.exceptions.RequestException as e:
        print(f"Error while fetching cryptocurrency value: {e}")
        return None
    
def get_reachedThresholds(id, cryptoValue):
    try:
        url = "https://bobagi.net/api/cryptoAlert/reachedThresholds"
        params = {"id": id, "cryptoValue": cryptoValue}
        response = requests.get(url, params=params)
        response.raise_for_status()
        return response.json()
    except requests.exceptions.RequestException as e:
        print(f"Error while fetching reached thresholds: {e}")
        return []

cycle = 0
while True:
    cycle += 1
    print(f"{Fore.YELLOW}Running cycle {cycle}!{Style.RESET_ALL}")
   
    clear_alerts()
   
    cryptos = get_cryptos()
    print(f"Total cryptos returned: {len(cryptos)}")
    print(f"cryptos returned: {cryptos}")
    for id, cryptoId in cryptos.items():
        cryptoValue = get_crypto_value(cryptoId)
        emails_to_send = get_reachedThresholds(id, cryptoValue)
        
        for threshold, greaterthancurrent, email in emails_to_send.items():    
            if greaterthancurrent:
                subject = f"{cryptoId} Alert - Price went up"
            else:
                subject = f"{cryptoId} Alert - Price went down"

            body = f"The value of {cryptoId} reached the threshold of {threshold}. Current value: {cryptoValue}"           
            send_email(subject, body, email)
            print(f"Alert email of {cryptoId} sent to {email}!")

    time.sleep(600)  # Sleep for 10 minutes
