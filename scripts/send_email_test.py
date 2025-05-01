import sys, os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import smtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from dotenv import load_dotenv
from logger_config import setup_logger

load_dotenv()
logger = setup_logger("send-email-test")

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
    server.sendmail(from_email, to_email, msg.as_string())
    server.quit()
    logger.info(f"Sent test email to {to_email}")

while True:
    subject = "Test Email from coin-watcher"
    body = "This is a test email."
    to_email = os.getenv("DESTINY_EMAIL")
    send_email(subject, body, to_email)
