# coin-alert

Python bot to watch coin value changes.

**Note:**  
- You must define an Email and its App Password in the `.env` file, along with the `DESTINY_EMAIL` variable which is the email that will receive the alert messages.
- This project was a test to implement the Coin Alert as seen on: [https://bobagi.click/CoinAlert](https://bobagi.net/CoinAlert).

## Install Python

If Python is not already installed on your VPS, you will need to install it. For example, on Ubuntu:

```bash
sudo apt-get update
sudo apt-get install python3
```

## Install pip

If pip is not already installed on your VPS, install it using:

```bash
sudo apt-get install python3-pip
```

## Important

For services like Gmail, you need to create an [App Password](https://myaccount.google.com/apppasswords).

## 1. Creating and Activating the Virtual Environment

Open your terminal and run:

```bash
python -m venv venv  # Creates the virtual environment
```

To activate the virtual environment:

- **Linux/Mac:**

    ```bash
    source venv/bin/activate
    ```

- **Windows:**

    ```bash
    source venv/Scripts/activate
    ```
    or
    ```bash
    .\venv\Scripts\Activate
    ```

To deactivate the environment, simply run:

```bash
deactivate
```

## 2. Installing Dependencies

First, upgrade pip:

```bash
python -m pip install --upgrade --force-reinstall pip
```

Then install the required packages with:

```bash
pip install -r requirements.txt
```

If you need to update your `requirements.txt` file, run:

```bash
pip freeze > requirements.txt
```

**Dependencies:**
- python-dotenv
- requests
- secure-smtplib (if applicable)
- Flask
- psycopg2-binary
- colorama

## 3. Running the Application

The project has been split into two main components:

1. **API Endpoints** (located in `app.py`):  
   This file contains the Flask API endpoints (e.g., `/test`, `/registerAlert`, etc.).

2. **Email Monitoring** (located in `scripts/send_email.py`):  
   This script handles the periodic checking of coin values and sends emails accordingly.

### To run the API:

```bash
python app.py
```

### To run the Email Monitoring script:

```bash
python scripts/send_email.py
```

> **Note:** In production, you might run the API with a WSGI server (like Gunicorn) instead of using Flask's development server.

## Additional Information

- **Email Alerts:**  
  The email monitoring script periodically (every 10 minutes) checks coin values and sends alerts based on threshold conditions stored in your PostgreSQL database.

- **Database Setup:**  
  Ensure that you have a PostgreSQL database running with the required tables. Use the provided Docker Compose file and `init.sql` if needed.





```bash
echo '
## 🛠 Como gerar e aplicar migrações do banco

### 1. Gerar um novo arquivo de migração automaticamente
Sempre que você adicionar, remover ou modificar modelos no `models.py`, gere uma nova migração com:

```bash
docker compose run --rm api alembic revision --autogenerate -m "mensagem_descrevendo_a_migracao"
```

> Exemplo:
> `docker compose run --rm api alembic revision --autogenerate -m "add user id at auto buy"`

---

### 2. Aplicar as migrações pendentes ao banco
Após gerar a migração, aplique ao banco com:

```bash
docker compose run --rm api alembic upgrade head
```

---

### 3. Resetar tudo (opcional)
Se quiser apagar o banco e tudo que foi gerado:

```bash
docker compose down -v
docker compose up --build
```
Isso reinicia a base e recria o banco do zero.
' >> README.md
```

Esse comando adiciona tudo direto no seu `README.md`. Quer que isso também rode `upgrade head` automaticamente no `entrypoint.sh` do container `api`?