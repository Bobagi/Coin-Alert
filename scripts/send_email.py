import sys, os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import smtplib
import time
import requests
import psycopg2
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from dotenv import load_dotenv
from logger_config import setup_logger

load_dotenv()
logger = setup_logger("send-email")

DB_HOST = os.getenv("DB_HOST")
DB_NAME = os.getenv("DB_NAME")
DB_USER = os.getenv("DB_USER")
DB_PASSWORD = os.getenv("DB_PASSWORD")
DB_PORT = os.getenv("DB_PORT", 5432)

conn = psycopg2.connect(
    host=DB_HOST,
    dbname=DB_NAME,
    user=DB_USER,
    password=DB_PASSWORD,
    port=DB_PORT
)
conn.autocommit = True

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
    logger.info(f"Sent email to {to_email}")

def clear_alerts_local():
    cur = conn.cursor()
    cur.execute("""
        DELETE FROM cripto_threshold 
        WHERE created_at IS NULL OR created_at < (NOW() - INTERVAL '1 WEEK')
    """)
    cur.execute("""
        WITH repeated_alerts AS (
            SELECT id, ROW_NUMBER() OVER (PARTITION BY id_cripto, id_email, greaterthancurrent ORDER BY created_at DESC) AS rn
            FROM cripto_threshold
        )
        DELETE FROM cripto_threshold
        WHERE id IN (SELECT id FROM repeated_alerts WHERE rn > 1)
    """)
    cur.close()

def clear_alert_by_id_local(alert_id):
    cur = conn.cursor()
    cur.execute("DELETE FROM cripto_threshold WHERE id = %s", (alert_id,))
    cur.close()

def get_cryptos_local():
    cur = conn.cursor()
    cur.execute("SELECT id, cryptoId AS cryptoid FROM cripto_currency")
    rows = cur.fetchall()
    cryptos = [{"id": row[0], "cryptoid": row[1]} for row in rows]
    cur.close()
    return cryptos

def get_crypto_value_local(crypto):
    try:
        url = f"https://api.coingecko.com/api/v3/simple/price?ids={crypto}&vs_currencies=usd"
        response = requests.get(url)
        response.raise_for_status()
        value = response.json()[crypto]["usd"]
        logger.info(f"Returned {crypto} value: {value}")
        return value
    except requests.exceptions.RequestException as e:
        logger.error(f"Error fetching {crypto} value: {e}")
        return None

def get_reached_thresholds_local(crypto_id, crypto_value):
    logger.info(f"Checking thresholds for crypto id: {crypto_id}, value: {crypto_value}")
    cur = conn.cursor()
    cur.execute("""
        SELECT ct.id, ct.threshold, ct.greaterthancurrent, ce.email
        FROM cripto_threshold ct
        INNER JOIN cripto_currency cc ON cc.id = ct.id_cripto
        INNER JOIN cripto_email ce ON ce.id = ct.id_email
        WHERE ct.id_cripto = %s
        AND CASE 
            WHEN ct.greaterthancurrent = TRUE THEN ct.threshold <= %s
            ELSE ct.threshold >= %s
        END
    """, (crypto_id, crypto_value, crypto_value))
    rows = cur.fetchall()
    thresholds = [{"id": row[0], "threshold": row[1], "greaterthancurrent": row[2], "email": row[3]} for row in rows]
    cur.close()
    logger.info(f"Returned thresholds: {thresholds}")
    return thresholds

def run_email_monitor():
    cycle = 0
    while True:
        cycle += 1
        logger.info(f"Running cycle {cycle}...")
        clear_alerts_local()
        cryptos = get_cryptos_local()
        logger.info(f"Total cryptos: {len(cryptos)}")
        for crypto in cryptos:
            crypto_id = crypto['id']
            crypto_name = crypto['cryptoid']
            logger.info(f"Processing crypto id: {crypto_id}, name: {crypto_name}")
            value = get_crypto_value_local(crypto_name)
            if value is None:
                continue
            alerts = get_reached_thresholds_local(crypto_id, value)
            for alert in alerts:
                alert_id = alert['id']
                threshold = alert['threshold']
                greater = alert['greaterthancurrent']
                email = alert['email']
                subject = f"{crypto_name} Alert - Price went {'up' if greater else 'down'}"
                body = f"The value of {crypto_name} reached the threshold of {threshold}. Current value: {value}"
                send_email(subject, body, email)
                logger.info(f"Sent alert email for {crypto_name} to {email}")
                clear_alert_by_id_local(alert_id)
        time.sleep(60)

if __name__ == "__main__":
    run_email_monitor()
