# Coin Alert (Go)

Aplicação web para registrar operações de criptomoedas e emitir alertas por e-mail, agora reescrita em Go seguindo princípios SOLID.

## Visão geral
- API e frontend servidos pela mesma aplicação Go.
- Banco PostgreSQL para persistir transações e alertas enviados.
- Automação interna para compras e vendas programadas, configuradas por intervalos.
- Container Docker único para a aplicação e um container para o banco, orquestrados via `docker-compose`.

## Variáveis de ambiente
Crie um arquivo `.env` com os parâmetros abaixo (valores de exemplo):

```
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=coin_alert
DB_HOST=db
DB_PORT=5432
EXTERNAL_DB_PORT=5432
API_PORT=5020
API_URL=http://localhost:5020

AUTO_SELL_INTERVAL_MINUTES=60
DAILY_PURCHASE_INTERVAL_MINUTES=1440

EMAIL_SENDER_ADDRESS=alertas@dominio.com
EMAIL_SENDER_PASSWORD=sua_senha
EMAIL_SMTP_HOST=smtp.dominio.com
EMAIL_SMTP_PORT=587
```

## Uso com Docker
1. Construa e suba os containers:
   ```
   docker compose up --build
   ```
2. Acesse `http://localhost:${API_PORT:-5020}` (ou o valor configurado em `API_PORT`) para visualizar o painel.

O serviço `app` só inicia após o Postgres estar saudável. O schema é criado automaticamente na inicialização.

### Ajustando usuário e senha do banco após já ter dados
Se você alterar `DB_USER` ou `DB_PASSWORD` em um ambiente que já possui o volume `db_data` inicializado, o Postgres continuará usando as credenciais originais gravadas no volume. Para aplicar novas credenciais, remova o volume antes de subir novamente os containers:

```
docker compose down -v
docker compose up --build
```

Alternativamente, mantenha as mesmas credenciais usadas na primeira inicialização do banco.

## Estrutura de pastas
- `cmd/server`: ponto de entrada da aplicação.
- `internal/config`: carregamento de configuração via ambiente.
- `internal/database`: conexão e migração simples de schema.
- `internal/domain`: modelos de domínio.
- `internal/repository`: persistência em PostgreSQL.
- `internal/service`: regras de negócio e automações.
- `internal/httpserver`: handlers HTTP e templates.
- `templates`: frontend em HTML/CSS.

## Funcionalidades
- Registrar compras e vendas com validações.
- Listar últimas operações.
- Enviar alertas por e-mail (SMTP autenticado) com registro no banco.
- Rotinas automáticas de compra e venda em intervalos configuráveis.
