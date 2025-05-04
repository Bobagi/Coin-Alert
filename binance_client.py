import os
from dotenv import load_dotenv
from binance.client import Client
from binance.enums import (
    SIDE_BUY,
    SIDE_SELL,
    ORDER_TYPE_MARKET,
    ORDER_TYPE_LIMIT,
    TIME_IN_FORCE_GTC
)
from decimal import Decimal
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker, Session
from models import UserCredentials

load_dotenv()

USE_TESTNET = os.getenv("BINANCE_TESTNET", "false").lower() == "true"

DATABASE_URL = os.getenv("DATABASE_URL")
engine = create_engine(DATABASE_URL)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

def get_binance_client(user_id: int, db_session: Session = None):
    close_session = False
    if db_session is None:
        db_session = SessionLocal()
        close_session = True

    try:
        user = db_session.query(UserCredentials).filter_by(id=user_id).first()
        if not user:
            raise ValueError(f"User with id {user_id} not found.")

        if USE_TESTNET:
            client = Client(api_key=user.testnet_api_key, api_secret=user.testnet_api_secret)
            client.API_URL = 'https://testnet.binance.vision/api'
        else:
            client = Client(api_key=user.api_key, api_secret=user.api_secret)
        return client
    finally:
        if close_session:
            db_session.close()

def get_asset_balance(asset: str, user_id: int, db_session: Session = None):
    print("received user_id: ", user_id)
    client = get_binance_client(user_id, db_session)
    print("Successfully got client binance")
    try:
        info = client.get_account()
        bal = next((b for b in info["balances"] if b["asset"] == asset.upper()), None)
        if bal is None:
            return {"status": "error", "message": f"Asset {asset} not found"}
        return {"status": "success", f"{asset.upper()}_balance": bal}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def place_market_order(symbol: str, side: str, user_id: int, db_session: Session = None, quantity: float = None, quoteOrderQty: float = None):
    client = get_binance_client(user_id, db_session)
    params = {
        "symbol": symbol,
        "side": SIDE_BUY if side.upper() == "BUY" else SIDE_SELL,
        "type": ORDER_TYPE_MARKET
    }
    if quoteOrderQty is not None:
        params["quoteOrderQty"] = format(Decimal(str(quoteOrderQty)), 'f')
    else:
        params["quantity"] = format(Decimal(str(quantity)), 'f')

    try:
        order = client.create_order(**params)
        return {"status": "success", "order": order, "onTestnet": USE_TESTNET}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def place_limit_order(symbol: str, side: str, quantity: float, price: float, user_id: int, db_session: Session = None):
    client = get_binance_client(user_id, db_session)
    qty_str = format(Decimal(str(quantity)), 'f')
    price_str = format(Decimal(str(price)), 'f')
    try:
        order = client.create_order(
            symbol=symbol,
            side=SIDE_SELL if side.upper() == "SELL" else SIDE_BUY,
            type=ORDER_TYPE_LIMIT,
            timeInForce=TIME_IN_FORCE_GTC,
            quantity=qty_str,
            price=price_str
        )
        return {"status": "success", "order": order, "onTestnet": USE_TESTNET}
    except Exception as e:
        return {"status": "error", "message": str(e)}
