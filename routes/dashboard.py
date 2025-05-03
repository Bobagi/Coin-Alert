import os
from flask import Blueprint, render_template
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker, joinedload
from models import CriptoThreshold

DB_HOST = os.getenv("DB_HOST")
DB_NAME = os.getenv("DB_NAME")
DB_USER = os.getenv("DB_USER")
DB_PASSWORD = os.getenv("DB_PASSWORD")
DB_PORT = os.getenv("DB_PORT", 5432)

dashboard_bp = Blueprint("dashboard", __name__)

@dashboard_bp.route('/dashboard')
def dashboard():
    # Configuração da conexão com o banco de dados
    SQLALCHEMY_DATABASE_URL = f"postgresql://{DB_USER}:{DB_PASSWORD}@{DB_HOST}:{DB_PORT}/{DB_NAME}"
    engine = create_engine(SQLALCHEMY_DATABASE_URL)
    SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
    db_session = SessionLocal()

    # Consulta aos dados de CriptoThreshold com os relacionamentos
    data = (
        db_session.query(CriptoThreshold)
        .options(joinedload(CriptoThreshold.email), joinedload(CriptoThreshold.cripto))
        .all()
    )

    # Estruturação dos dados para o template
    thresholds = [{
        "email": t.email.email,
        "symbol": t.cripto.symbol,
        "threshold": float(t.threshold),
        "greaterThanCurrent": t.greaterThanCurrent
    } for t in data]

    # Caminho para o arquivo de log
    basedir = os.path.abspath(os.path.dirname(__file__))
    log_path = os.getenv("LOG_PATH", os.path.join(basedir, "..", "logs", "send-email.log"))

    # Leitura dos últimos 50 logs
    if os.path.exists(log_path):
        with open(log_path, 'r') as f:
            logs = "\n".join(f.readlines()[-50:])
    else:
        logs = "Log file not found."

    # Renderização do template com os dados
    return render_template("dashboard.html", thresholds=thresholds, logs=logs)
