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
        print(f"Alerts cleared response: {response}")
        print("Alerts cleared successfully!")
    except requests.exceptions.RequestException as e:
        print(f"Error while clearing alerts: {e}")

def clear_alert_by_id(id):
    print(f"Cleaning alert by id {id}...")
    url = "https://bobagi.net/api/cryptoAlert/clearAlertById"
    try:      
        payload = {"id": id}
        
        response = requests.post(url, json=payload)
        response.raise_for_status()       
        print("Alert deleted successfully!")
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
    print(f"Getting {crypto} value...")
    try:
        url = f"https://api.coingecko.com/api/v3/simple/price?ids={crypto}&vs_currencies=usd"
        response = requests.get(url)
        response.raise_for_status()
        print(f"Returned {crypto} value: {Fore.YELLOW}{response.json()[crypto]["usd"]}{Style.RESET_ALL}")
        return response.json()[crypto]["usd"]
    except requests.exceptions.RequestException as e:
        print(f"Error while fetching cryptocurrency value: {e}")
        return None
    
def get_reachedThresholds(id, cryptoValue):
    print(f"Looking for reached thresholds...")
    try:
        url = "https://bobagi.net/api/cryptoAlert/reachedThresholds"
        params = {"id": id, "cryptoValue": cryptoValue}
        response = requests.get(url, params=params)
        response.raise_for_status()
        
        reached_thresholds = response.json()
        # emails = [threshold['email'] for threshold in reached_thresholds]
        
        return reached_thresholds
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
    for crypto in cryptos:
        id_value = crypto['id']
        cryptoid_value = crypto['cryptoid']

        print(f"cryptos id: {id_value}")
        print(f"cryptos cryptoid: {cryptoid_value}")
               
        cryptoValue = get_crypto_value(cryptoid_value)
        emails_to_send = get_reachedThresholds(id_value, cryptoValue)
        
        # print(f"emails_to_send: {emails_to_send}")
        for email_to_send in emails_to_send:    
            id = email_to_send['id']
            threshold = email_to_send['threshold']
            greaterthancurrent = email_to_send['greaterthancurrent']
            email = email_to_send['email']
        
            if greaterthancurrent:
                subject = f"{cryptoid_value} Alert - Price went up"
            else:
                subject = f"{cryptoid_value} Alert - Price went down"

            body = f"The value of {cryptoid_value} reached the threshold of {threshold}. Current value: {cryptoValue}"           
            send_email(subject, body, email)
            print(f"Alert email of {cryptoid_value} sent to {email}!")
            clear_alert_by_id(id)
       
    time.sleep(600)  # Sleep for 10 minutes
