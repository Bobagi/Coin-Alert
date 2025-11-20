import os
from typing import List

import requests
from decimal import Decimal
from flask import Blueprint, jsonify, render_template, request
from sqlalchemy import create_engine, func
from sqlalchemy.orm import joinedload, sessionmaker
from models import (
    AutoBuyQuota,
    AutoPositions,
    CriptoCurrency,
    CriptoThreshold,
    DailyPurchaseConfig,
    Trades,
    UserCredentials,
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


def find_user_by_email(session, email):
    return session.query(UserCredentials).filter(UserCredentials.email == email).first()


def ensure_user_exists(session, email):
    user = find_user_by_email(session, email)
    if not user:
        raise ValueError(f"Usuário com email {email} não encontrado")
    return user


def parse_decimal_value(value, field_name):
    try:
        return Decimal(str(value))
    except Exception as error:  # noqa: BLE001
        raise ValueError(f"Valor inválido para {field_name}") from error


def ensure_symbol_exists(session, symbol):
    symbol_exists = session.query(CriptoCurrency).filter(CriptoCurrency.symbol == symbol).first()
    if not symbol_exists:
        raise ValueError(
            "Símbolo não encontrado. Cadastre o token na seção de alertas ou escolha um símbolo já existente."
        )
    return symbol_exists


def build_symbol_options(session):
    symbols: List[str] = [row.symbol for row in session.query(CriptoCurrency).order_by(CriptoCurrency.symbol.asc())]
    return symbols


def fetch_usdt_price_brl():
    try:
        response = requests.get("https://api.binance.com/api/v3/ticker/price", params={"symbol": "USDTBRL"}, timeout=5)
        response.raise_for_status()
        return Decimal(response.json().get("price", "0"))
    except Exception:  # noqa: BLE001
        return None


def compute_minimum_daily_amount_brl():
    minimum_usdt = Decimal(os.getenv("MINIMUM_ORDER_USDT", "10"))
    conversion_price = fetch_usdt_price_brl()
    if conversion_price is None:
        fallback_brl = os.getenv("DAILY_SPEND_BRL", "10")
        return float(Decimal(fallback_brl))
    return float(minimum_usdt * conversion_price)


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
        "minimumPerOrderBrl": compute_minimum_daily_amount_brl(),
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
    auto_buy_log_path = os.path.join(log_dir, "auto-buy.log")
    return {
        "emails": tail_log(email_log_path),
        "daily": tail_log(daily_log_path),
        "autoSell": tail_log(auto_sell_log_path),
        "autoBuy": tail_log(auto_buy_log_path),
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
        "availableSymbols": build_symbol_options(session),
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


@dashboard_bp.route('/dashboard/crypto-symbols')
def crypto_symbols():
    db_session = create_database_session()
    try:
        symbols = build_symbol_options(db_session)
    finally:
        db_session.close()
    return jsonify({"symbols": symbols})


def handle_configuration_action(action_function):
    database_session = create_database_session()
    try:
        result = action_function(database_session)
        database_session.commit()
        return jsonify({"status": "success", **result})
    except ValueError as validation_error:
        database_session.rollback()
        return jsonify({"status": "error", "message": str(validation_error)}), 400
    except Exception as unexpected_error:  # noqa: BLE001
        database_session.rollback()
        return jsonify({"status": "error", "message": "Erro inesperado ao processar a configuração."}), 500
    finally:
        database_session.close()


def validate_request_fields(payload, required_fields):
    missing_fields = [field for field in required_fields if field not in payload or payload[field] in (None, "")]
    if missing_fields:
        raise ValueError(f"Campos obrigatórios ausentes: {', '.join(missing_fields)}")


@dashboard_bp.route('/dashboard/config/auto-buy', methods=['POST'])
def create_or_update_auto_buy_quota():
    return handle_configuration_action(_process_auto_buy_payload)


def _process_auto_buy_payload(database_session):
    payload = request.get_json(silent=True) or {}
    validate_request_fields(payload, ["email", "symbol", "quotaLimitBrl"])

    user = ensure_user_exists(database_session, payload["email"])
    ensure_symbol_exists(database_session, payload["symbol"])
    quota_limit_value = parse_decimal_value(payload["quotaLimitBrl"], "quotaLimitBrl")

    quota = (
        database_session.query(AutoBuyQuota)
        .filter(AutoBuyQuota.user_id == user.id, AutoBuyQuota.crypto_symbol == payload["symbol"])
        .first()
    )

    if quota:
        quota.quota_limit_brl = quota_limit_value
    else:
        quota = AutoBuyQuota(
            user_id=user.id,
            crypto_symbol=payload["symbol"],
            quota_limit_brl=quota_limit_value,
            quota_used_brl=Decimal("0"),
        )
        database_session.add(quota)
    database_session.flush()

    return {"message": "Auto buy atualizado com sucesso", "quotaId": quota.id}


@dashboard_bp.route('/dashboard/config/auto-sell', methods=['POST'])
def create_or_update_auto_sell_quota():
    return handle_configuration_action(_process_auto_sell_payload)


def _process_auto_sell_payload(database_session):
    payload = request.get_json(silent=True) or {}
    validate_request_fields(payload, ["email", "symbol", "sellLimitBrl"])

    user = ensure_user_exists(database_session, payload["email"])
    ensure_symbol_exists(database_session, payload["symbol"])
    sell_limit_value = parse_decimal_value(payload["sellLimitBrl"], "sellLimitBrl")

    quota = (
        database_session.query(AutoBuyQuota)
        .filter(AutoBuyQuota.user_id == user.id, AutoBuyQuota.crypto_symbol == payload["symbol"])
        .first()
    )

    if quota:
        quota.quota_limit_brl = sell_limit_value
    else:
        quota = AutoBuyQuota(
            user_id=user.id,
            crypto_symbol=payload["symbol"],
            quota_limit_brl=sell_limit_value,
            quota_used_brl=Decimal("0"),
        )
        database_session.add(quota)
    database_session.flush()

    return {"message": "Auto sell configurado com sucesso", "quotaId": quota.id}


@dashboard_bp.route('/dashboard/config/daily-buy', methods=['POST'])
def create_daily_buy_config():
    return handle_configuration_action(_process_daily_buy_payload)


def _process_daily_buy_payload(database_session):
    payload = request.get_json(silent=True) or {}
    validate_request_fields(payload, ["email", "symbol", "amountBrl"])

    user = ensure_user_exists(database_session, payload["email"])
    ensure_symbol_exists(database_session, payload["symbol"])
    amount_value = parse_decimal_value(payload["amountBrl"], "amountBrl")

    daily_config = DailyPurchaseConfig(
        user_id=user.id,
        crypto_symbol=payload["symbol"],
        amount_brl=amount_value,
    )
    database_session.add(daily_config)
    database_session.flush()

    return {"message": "Compra diária registrada", "configId": daily_config.id}
