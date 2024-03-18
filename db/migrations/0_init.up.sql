CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY,
    login varchar UNIQUE NOT NULL,
    password varchar NOT NULL
);

CREATE TABLE IF NOT EXISTS orders (
    id uuid PRIMARY KEY,
    order_number varchar UNIQUE NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status varchar NOT NULL,
    accrual numeric(11, 3) NOT NULL,
    uploaded_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS withdrawals (
    id uuid PRIMARY KEY,
    order_number varchar UNIQUE NOT NULL,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sum numeric(11, 3) NOT NULL,
    status varchar NOT NULL DEFAULT 'FAILURE',
    processed_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS balances (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance numeric(11, 3) NOT NULL DEFAULT 0,
    updated_at timestamp DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS withdraw_balances (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    sum numeric(11, 3) NOT NULL DEFAULT 0,
    updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);
