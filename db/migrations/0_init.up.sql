CREATE TABLE IF NOT EXISTS users (
    id uuid NOT NULL,
    login varchar NOT NULL,
    password varchar NOT NULL,
    PRIMARY KEY (id),
    UNIQUE (login)
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
    user_id uuid UNIQUE NOT NULL REFERENCES users(id),
    status varchar NOT NULL,
    accrual integer,
    uploaded_at timestamp DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS withdraws (
    id uuid NOT NULL,
    order_id uuid UNIQUE NOT NULL REFERENCES orders(id),
    user_id uuid UNIQUE NOT NULL REFERENCES users(id),
    status varchar NOT NULL,
    accrual integer,
    processed_at timestamp DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS accruals (
     id uuid NOT NULL,
     order_id uuid UNIQUE NOT NULL REFERENCES orders(id),
     user_id uuid UNIQUE NOT NULL REFERENCES users(id),
     status varchar NOT NULL,
     accrual integer,
     processed_at timestamp DEFAULT CURRENT_TIMESTAMP,
     PRIMARY KEY (id)
);