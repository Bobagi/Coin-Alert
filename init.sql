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
