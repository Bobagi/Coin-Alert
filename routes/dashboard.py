import os
from flask import Blueprint, render_template
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker, joinedload
from models import (
    CriptoThreshold,
    DailyPurchaseConfig,
    AutoBuyQuota,
    UserCredentials,
)

DB_HOST = os.getenv("DB_HOST")
DB_NAME = os.getenv("DB_NAME")
DB_USER = os.getenv("DB_USER")
DB_PASSWORD = os.getenv("DB_PASSWORD")
DB_PORT = os.getenv("DB_PORT", 5432)

dashboard_bp = Blueprint("dashboard", __name__)

@dashboard_bp.route('/dashboard')
def dashboard():
    # Database connection setup
    SQLALCHEMY_DATABASE_URL = f"postgresql://{DB_USER}:{DB_PASSWORD}@{DB_HOST}:{DB_PORT}/{DB_NAME}"
    engine = create_engine(SQLALCHEMY_DATABASE_URL)
    SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
    db_session = SessionLocal()

    # Query CriptoThreshold data with related objects
    data = (
        db_session.query(CriptoThreshold)
        .options(joinedload(CriptoThreshold.email), joinedload(CriptoThreshold.cripto))
        .all()
    )

    # Prepare data for the template
    thresholds = [
        {
            "email": t.email.email,
            "symbol": t.cripto.symbol,
            "threshold": float(t.threshold),
            "greaterThanCurrent": t.greaterThanCurrent,
        }
        for t in data
    ]

    # Daily buy configuration
    daily_data = (
        db_session.query(DailyPurchaseConfig)
        .options(joinedload(DailyPurchaseConfig.user))
        .all()
    )
    daily_configs = [
        {
            "email": d.user.email,
            "symbol": d.crypto_symbol,
            "amount_brl": float(d.amount_brl),
        }
        for d in daily_data
    ]
    dip_hour_utc = int(os.getenv("DIP_HOUR_UTC", "4"))
    dip_hour_brl = (dip_hour_utc - 3) % 24
    daily_spend_brl = float(os.getenv("DAILY_SPEND_BRL", "0"))

    # Auto sell quotas
    quota_data = (
        db_session.query(AutoBuyQuota)
        .options(joinedload(AutoBuyQuota.user))
        .all()
    )
    auto_sell_quotas = [
        {
            "email": q.user.email,
            "symbol": q.crypto_symbol,
            "limit": float(q.quota_limit_brl),
            "used": float(q.quota_used_brl),
        }
        for q in quota_data
    ]

    # Log file paths
    basedir = os.path.abspath(os.path.dirname(__file__))
    log_dir = os.getenv("LOG_DIR", os.path.join(basedir, "..", "logs"))
    email_log_path = os.getenv("LOG_PATH", os.path.join(log_dir, "send-email.log"))
    daily_log_path = os.path.join(log_dir, "daily-buy.log")
    auto_sell_log_path = os.path.join(log_dir, "auto-sell.log")

    def tail_log(path, lines=50):
        if os.path.exists(path):
            with open(path, "r") as f:
                return "\n".join(f.readlines()[-lines:])
        return "Log file not found."

    logs = tail_log(email_log_path)
    daily_logs = tail_log(daily_log_path)
    auto_sell_logs = tail_log(auto_sell_log_path)

    # Render the template with data
    return render_template(
        "dashboard.html",
        thresholds=thresholds,
        logs=logs,
        daily_configs=daily_configs,
        dip_hour_utc=dip_hour_utc,
        dip_hour_brl=dip_hour_brl,
        daily_spend_brl=daily_spend_brl,
        daily_logs=daily_logs,
        auto_sell_quotas=auto_sell_quotas,
        auto_sell_logs=auto_sell_logs,
    )
