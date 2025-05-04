from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy import Column, Integer, String, Numeric, Boolean, DateTime, ForeignKey, BigInteger, func
from sqlalchemy.orm import relationship

Base = declarative_base()

class CriptoCurrency(Base):
    __tablename__ = 'cripto_currency'
    id = Column(Integer, primary_key=True)
    symbol = Column(String(50), unique=True, nullable=False)
    crypto_id = Column(String(255), nullable=False)

class CriptoEmail(Base):
    __tablename__ = 'cripto_email'
    id = Column(Integer, primary_key=True)
    email = Column(String(255), unique=True, nullable=False)

class CriptoThreshold(Base):
    __tablename__ = 'cripto_threshold'
    id = Column(Integer, primary_key=True)
    id_email = Column(Integer, ForeignKey('cripto_email.id', ondelete="CASCADE"), nullable=False)
    id_cripto = Column(Integer, ForeignKey('cripto_currency.id', ondelete="CASCADE"), nullable=False)
    threshold = Column(Numeric, nullable=False)
    greater_than_current = Column(Boolean, nullable=False)
    created_at = Column(DateTime, nullable=False, server_default=func.now())

    email = relationship("CriptoEmail", backref="thresholds")
    cripto = relationship("CriptoCurrency", backref="thresholds")

class Trades(Base):
    __tablename__ = 'trades'
    id = Column(Integer, primary_key=True)
    order_id = Column(BigInteger, unique=True, nullable=False)
    on_testnet = Column(Boolean, nullable=False)
    client_order_id = Column(String(100), nullable=False)
    symbol = Column(String(20), nullable=False)
    side = Column(String(4), nullable=False)
    qty = Column(Numeric, nullable=False)
    quote_qty = Column(Numeric, nullable=False)
    price = Column(Numeric)
    status = Column(String(20), nullable=False)
    created_at = Column(DateTime, nullable=False, server_default=func.now())
    operation_type = Column(String(5))

class AutoPositions(Base):
    __tablename__ = 'auto_positions'
    trade_id = Column(BigInteger, primary_key=True)
    purchase_date = Column(DateTime, nullable=False)
    sell_date = Column(DateTime)
    sell_trade_id = Column(BigInteger, ForeignKey('trades.order_id'))

    sell_trade = relationship("Trades", foreign_keys=[sell_trade_id])

class UserCredentials(Base):
    __tablename__ = 'user_credentials'
    id = Column(Integer, primary_key=True)
    email = Column(String(255), unique=True, nullable=False)
    api_key = Column(String(255), nullable=False)
    api_secret = Column(String(255), nullable=False)
    testnet_api_key = Column(String(255), nullable=True)
    testnet_api_secret = Column(String(255), nullable=True)

class DailyPurchaseConfig(Base):
    __tablename__ = 'daily_purchase_config'
    id = Column(Integer, primary_key=True)
    user_id = Column(Integer, ForeignKey('user_credentials.id', ondelete="CASCADE"), nullable=False)
    crypto_symbol = Column(String(50), nullable=False)
    amount_brl = Column(Numeric, nullable=False)
    created_at = Column(DateTime, nullable=False, server_default=func.now())

    user = relationship("UserCredentials", backref="daily_purchase_configs")

class AutoBuyQuota(Base):
    __tablename__ = 'auto_buy_quota'
    id = Column(Integer, primary_key=True)
    user_id = Column(Integer, ForeignKey('user_credentials.id', ondelete="CASCADE"), nullable=False)
    quota_limit_brl = Column(Numeric, nullable=False)
    quota_used_brl = Column(Numeric, nullable=False, default=0)
    updated_at = Column(DateTime, nullable=False, server_default=func.now(), onupdate=func.now())

    user = relationship("UserCredentials", backref="auto_buy_quota")
