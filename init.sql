-- Table cripto_currency
CREATE TABLE IF NOT EXISTS cripto_currency (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(50) UNIQUE NOT NULL,
    cryptoId VARCHAR(255) NOT NULL
);

-- Table cripto_email
CREATE TABLE IF NOT EXISTS cripto_email (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL
);

-- Table cripto_threshold
CREATE TABLE IF NOT EXISTS cripto_threshold (
    id SERIAL PRIMARY KEY,
    id_email INTEGER NOT NULL,
    id_cripto INTEGER NOT NULL,
    threshold DECIMAL NOT NULL,
    greaterThanCurrent BOOLEAN NOT NULL,
    created_at TIMESTAMP NOT NULL,
    FOREIGN KEY (id_email) REFERENCES cripto_email(id) ON DELETE CASCADE,
    FOREIGN KEY (id_cripto) REFERENCES cripto_currency(id) ON DELETE CASCADE
);

-- Table trades
CREATE TABLE IF NOT EXISTS trades (
    id               SERIAL PRIMARY KEY,
    order_id         BIGINT   NOT NULL,
    on_testnet       BOOLEAN  NOT NULL,
    client_order_id  VARCHAR(100) NOT NULL,
    symbol           VARCHAR(20)  NOT NULL,
    side             VARCHAR(4)   NOT NULL,
    qty              NUMERIC      NOT NULL,
    quote_qty        NUMERIC      NOT NULL,
    price            NUMERIC,
    status           VARCHAR(20)  NOT NULL,
    created_at       TIMESTAMP    NOT NULL DEFAULT NOW()
);

-- Table auto_positions
CREATE TABLE IF NOT EXISTS auto_positions (
    trade_id      BIGINT PRIMARY KEY,
    purchase_date TIMESTAMP NOT NULL,
    sell_date     TIMESTAMP
);