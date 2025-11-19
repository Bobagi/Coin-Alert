import os
from decimal import Decimal
from flask import Blueprint, jsonify, render_template
from sqlalchemy import create_engine, func
from sqlalchemy.orm import joinedload, sessionmaker
from models import (
    AutoBuyQuota,
    AutoPositions,
    CriptoThreshold,
    DailyPurchaseConfig,
    Trades,
)

DB_HOST = os.getenv("DB_HOST")
DB_NAME = os.getenv("DB_NAME")
DB_USER = os.getenv("DB_USER")
DB_PASSWORD = os.getenv("DB_PASSWORD")
DB_PORT = os.getenv("DB_PORT", 5432)

dashboard_bp = Blueprint("dashboard", __name__)


def create_database_session():
    database_url = f"postgresql://{DB_USER}:{DB_PASSWORD}@{DB_HOST}:{DB_PORT}/{DB_NAME}"
    engine = create_engine(database_url)
    session_factory = sessionmaker(autocommit=False, autoflush=False, bind=engine)
    return session_factory()


def tail_log(path, lines=50):
    if os.path.exists(path):
        with open(path, "r") as file_pointer:
            return "\n".join(file_pointer.readlines()[-lines:])
    return "Log file not found."


def build_thresholds(session):
    data = (
        session.query(CriptoThreshold)
        .options(joinedload(CriptoThreshold.email), joinedload(CriptoThreshold.cripto))
        .all()
    )
    return [
        {
            "email": threshold.email.email,
            "symbol": threshold.cripto.symbol,
            "threshold": float(threshold.threshold),
            "greaterThanCurrent": threshold.greaterThanCurrent,
        }
        for threshold in data
    ]


def build_daily_configuration(session):
    daily_data = (
        session.query(DailyPurchaseConfig)
        .options(joinedload(DailyPurchaseConfig.user))
        .all()
    )
    dip_hour_utc = int(os.getenv("DIP_HOUR_UTC", "4"))
    return {
        "configs": [
            {
                "email": config.user.email,
                "symbol": config.crypto_symbol,
                "amountBrl": float(config.amount_brl),
            }
            for config in daily_data
        ],
        "dipHourUtc": dip_hour_utc,
        "dipHourBrl": (dip_hour_utc - 3) % 24,
        "dailySpendBrl": float(os.getenv("DAILY_SPEND_BRL", "0")),
    }


def get_auto_buy_positions(session, user_id, symbol):
    open_position_value = (
        session.query(func.coalesce(func.sum(Trades.quote_qty), 0))
        .join(AutoPositions, AutoPositions.trade_id == Trades.order_id)
        .filter(
            AutoPositions.user_id == user_id,
            Trades.symbol == symbol,
            AutoPositions.sell_date.is_(None),
        )
        .scalar()
    )
    open_position_count = (
        session.query(AutoPositions)
        .join(Trades, Trades.order_id == AutoPositions.trade_id)
        .filter(
            AutoPositions.user_id == user_id,
            Trades.symbol == symbol,
            AutoPositions.sell_date.is_(None),
        )
        .count()
    )
    last_buy = (
        session.query(AutoPositions, Trades)
        .join(Trades, Trades.order_id == AutoPositions.trade_id)
        .filter(AutoPositions.user_id == user_id, Trades.symbol == symbol)
        .order_by(AutoPositions.purchase_date.desc())
        .first()
    )
    last_sell = (
        session.query(AutoPositions, Trades)
        .join(Trades, Trades.order_id == AutoPositions.sell_trade_id)
        .filter(
            AutoPositions.user_id == user_id,
            Trades.symbol == symbol,
            AutoPositions.sell_date.isnot(None),
        )
        .order_by(AutoPositions.sell_date.desc())
        .first()
    )
    last_release = (
        session.query(func.max(AutoPositions.sell_date))
        .join(Trades, Trades.order_id == AutoPositions.sell_trade_id)
        .filter(
            AutoPositions.user_id == user_id,
            Trades.symbol == symbol,
            AutoPositions.sell_date.isnot(None),
        )
        .scalar()
    )
    return {
        "lockedExposure": float(Decimal(open_position_value or 0)),
        "openPositions": open_position_count,
        "lastBuy": serialize_auto_position(last_buy, is_sell=False),
        "lastSell": serialize_auto_position(last_sell, is_sell=True),
        "lastReleaseAt": last_release.isoformat() if last_release else None,
    }


def serialize_auto_position(position_row, is_sell):
    if not position_row:
        return None
    auto_position, trade = position_row
    reference_date = auto_position.sell_date if is_sell else auto_position.purchase_date
    return {
        "orderId": trade.order_id,
        "status": trade.status,
        "quoteQty": float(Decimal(trade.quote_qty or 0)),
        "price": float(Decimal(trade.price)) if trade.price is not None else None,
        "eventDate": reference_date.isoformat() if reference_date else None,
        "side": trade.side,
    }


def build_auto_buy_summary(session):
    quotas = session.query(AutoBuyQuota).options(joinedload(AutoBuyQuota.user)).all()
    auto_buy_status = []
    for quota in quotas:
        quota_limit = Decimal(quota.quota_limit_brl)
        quota_used = Decimal(quota.quota_used_brl)
        positions = get_auto_buy_positions(session, quota.user_id, quota.crypto_symbol)
        auto_buy_status.append(
            {
                "email": quota.user.email,
                "symbol": quota.crypto_symbol,
                "quotaLimitBrl": float(quota_limit),
                "quotaUsedBrl": float(quota_used),
                "quotaRemainingBrl": float(max(quota_limit - quota_used, Decimal("0"))),
                **positions,
            }
        )
    return auto_buy_status


def build_auto_sell_quotas(session):
    quota_data = session.query(AutoBuyQuota).options(joinedload(AutoBuyQuota.user)).all()
    return [
        {
            "email": quota.user.email,
            "symbol": quota.crypto_symbol,
            "limit": float(quota.quota_limit_brl),
            "used": float(quota.quota_used_brl),
        }
        for quota in quota_data
    ]


def load_dashboard_logs():
    basedir = os.path.abspath(os.path.dirname(__file__))
    log_dir = os.getenv("LOG_DIR", os.path.join(basedir, "..", "logs"))
    email_log_path = os.getenv("LOG_PATH", os.path.join(log_dir, "send-email.log"))
    daily_log_path = os.path.join(log_dir, "daily-buy.log")
    auto_sell_log_path = os.path.join(log_dir, "auto-sell.log")
    return {
        "emails": tail_log(email_log_path),
        "daily": tail_log(daily_log_path),
        "autoSell": tail_log(auto_sell_log_path),
        "autoBuy": tail_log(auto_sell_log_path),
    }


def build_dashboard_payload(session):
    return {
        "thresholds": build_thresholds(session),
        "daily": build_daily_configuration(session),
        "autoSell": {
            "quotas": build_auto_sell_quotas(session),
        },
        "autoBuy": {
            "quotas": build_auto_buy_summary(session),
        },
        "logs": load_dashboard_logs(),
    }


@dashboard_bp.route('/dashboard')
def dashboard():
    db_session = create_database_session()
    try:
        dashboard_data = build_dashboard_payload(db_session)
    finally:
        db_session.close()

    return render_template("dashboard.html", dashboard_data=dashboard_data)


@dashboard_bp.route('/dashboard/data')
def dashboard_data():
    db_session = create_database_session()
    try:
        dashboard_data = build_dashboard_payload(db_session)
    finally:
        db_session.close()

    return jsonify(dashboard_data)
