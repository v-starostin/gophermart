CREATE TABLE IF NOT EXISTS users (
    id uuid,
    login varchar UNIQUE NOT NULL,
    password varchar NOT NULL,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS tokens (
    id uuid NOT NULL,
    user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    token varchar NOT NULL,
    PRIMARY KEY(id)
);

CREATE TABLE IF NOT EXISTS orders (
    id uuid NOT NULL,
    order_number varchar UNIQUE NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id),
    status varchar NOT NULL,
    accrual int NOT NULL,
    uploaded_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS withdraws (
    id uuid NOT NULL,
    order_number varchar UNIQUE NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id),
    sum int NOT NULL,
    status varchar NOT NULL DEFAULT 'FAILURE',
    processed_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS balances (
    user_id uuid PRIMARY KEY REFERENCES users(id),
    balance int NOT NULL DEFAULT 0,
    updated_at timestamp DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS withdraw_balances (
    user_id uuid PRIMARY KEY REFERENCES users(id),
    amount int NOT NULL DEFAULT 0,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);