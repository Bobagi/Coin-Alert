from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy import Column, Integer, String, Numeric, Boolean, DateTime, ForeignKey, BigInteger, func
from sqlalchemy.orm import relationship

Base = declarative_base()


class CriptoCurrency(Base):
    __tablename__ = 'cripto_currency'
    id = Column(Integer, primary_key=True)
    symbol = Column(String(50), unique=True, nullable=False)
    cryptoId = Column("crypto_id", String(255), nullable=False)


class CriptoEmail(Base):
    __tablename__ = 'cripto_email'
    id = Column(Integer, primary_key=True)
    email = Column(String(255), unique=True, nullable=False)


class CriptoThreshold(Base):
    __tablename__ = 'cripto_threshold'
    id = Column(Integer, primary_key=True)
    id_email = Column("id_email", Integer, ForeignKey('cripto_email.id', ondelete="CASCADE"), nullable=False)
    id_cripto = Column("id_cripto", Integer, ForeignKey('cripto_currency.id', ondelete="CASCADE"), nullable=False)
    threshold = Column(Numeric, nullable=False)
    greaterThanCurrent = Column("greater_than_current", Boolean, nullable=False)
    created_at = Column("created_at", DateTime, nullable=False)

    email = relationship("CriptoEmail")
    cripto = relationship("CriptoCurrency")


class Trades(Base):
    __tablename__ = 'trades'
    id = Column(Integer, primary_key=True)
    order_id = Column(BigInteger, unique=True, nullable=False)
    on_testnet = Column("on_testnet", Boolean, nullable=False)
    client_order_id = Column("client_order_id", String(100), nullable=False)
    symbol = Column(String(20), nullable=False)
    side = Column(String(4), nullable=False)
    qty = Column(Numeric, nullable=False)
    quote_qty = Column("quote_qty", Numeric, nullable=False)
    price = Column(Numeric)
    status = Column(String(20), nullable=False)
    created_at = Column("created_at", DateTime, nullable=False, server_default=func.now())
    operation_type = Column(String(5))

class AutoPositions(Base):
    __tablename__ = 'auto_positions'
    tradeId = Column("trade_id", BigInteger, primary_key=True)
    purchaseDate = Column("purchase_date", DateTime, nullable=False)
    sellDate = Column("sell_date", DateTime)
    sellTradeId = Column("sell_trade_id", BigInteger, ForeignKey('trades.order_id'))

    sell_trade = relationship("Trades", foreign_keys=[sellTradeId])
