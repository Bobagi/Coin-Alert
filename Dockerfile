FROM python:3.10-slim

WORKDIR /app

# Instalar as dependências do sistema necessárias para compilar o psycopg2
RUN apt-get update && apt-get install -y libpq-dev gcc

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

CMD ["python", "main.py"]
