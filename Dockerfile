FROM python:3.11-slim

ARG API_PORT=5000
ENV API_PORT=$API_PORT
ENV PYTHONUNBUFFERED=1

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
       build-essential libpq-dev \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY requirements.txt ./
RUN pip install --upgrade pip \
  && pip install --no-cache-dir -r requirements.txt

COPY . ./

ENTRYPOINT ["sh", "-c", "python migrate.py && exec \"$@\"", "--"]
CMD ["python", "app.py"]
