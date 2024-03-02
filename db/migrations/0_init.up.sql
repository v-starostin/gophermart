CREATE TABLE IF NOT EXISTS users (
    id uuid NOT NULL,
    login varchar NOT NULL,
    password varchar NOT NULL,
    PRIMARY KEY(id, login)
);

CREATE TABLE IF NOT EXISTS tokens (
    id uuid NOT NULL,
    user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    token varchar NOT NULL,
    PRIMARY KEY(id)
)