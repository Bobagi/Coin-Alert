# coin-alert

Python bot to watch coin value changes

Need to define an Email and it's App Password in the .env file, and in DESTINY the email that will receive the message.

That project was a test to implement the Coin Alert in: https://bobagi.net/CoinAlert

Install Python: If Python is not already installed on your VPS, you'll need to install it. You can do this using the package manager for your operating system. For example, on Ubuntu, you can install Python 3 with the following command:

bash
Copy code
sudo apt-get update
sudo apt-get install python3
Install pip: If pip is not already installed on your VPS, you'll need to install it. You can do this by running the following command:

bash
Copy code
sudo apt-get install python3-pip
Install any required dependencies: If your script requires any third-party packages, you'll need to install them using pip. For example, if your script uses the requests library, you can install it with the following command:

bash
Copy code
pip3 install requests

> **Importante:** Para serviços como Gmail, você precisará criar um [App Password](https://myaccount.google.com/apppasswords)

### 3. Configurar Ambiente Virtual
```bash
python -m venv venv
```

- **Ativar Ambiente:**
  ```bash
  # Linux/MacOS
  source venv/bin/activate
  
  # Windows
  .\venv\Scripts\activate
  ```

- **Desativar Ambiente:**
  ```bash
  deactivate
  ```

### 4. Instalar Dependências
```bash
pip install --upgrade pip
pip install -r requirements.txt
```

## ▶️ Execução
```bash
python main.py  # Substitua pelo nome real do seu arquivo principal
```

# Dependencies

To install depencendies run

`pip install -r requirements.txt`

python-dotenv
requests
secure-smtplib
