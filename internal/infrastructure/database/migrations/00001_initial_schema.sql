-- +goose Up
-- goose wraps each migration in a transaction, so no explicit BEGIN/COMMIT here.

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(36) NOT NULL UNIQUE,
    phone_number VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(255),
    is_blocked BOOLEAN NOT NULL DEFAULT FALSE,
    blocked_reason TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE sessions (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL REFERENCES users(public_id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    conversation_history JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX sessions_user_id_created_at_idx ON sessions (user_id, created_at DESC);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE agreements (
    id VARCHAR(36) PRIMARY KEY,
    initiator_id VARCHAR(36) NOT NULL REFERENCES users(public_id),
    counterparty_id VARCHAR(36) NOT NULL REFERENCES users(public_id),
    agreement_type VARCHAR(32) NOT NULL,
    description TEXT NOT NULL,
    amount_minor BIGINT NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'NGN',
    status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT agreements_status_check
        CHECK (status IN ('PENDING', 'CONFIRMED', 'REJECTED', 'CANCELLED'))
);

CREATE INDEX agreements_initiator_id_idx ON agreements (initiator_id);
CREATE INDEX agreements_counterparty_status_idx ON agreements (counterparty_id, status);
CREATE INDEX agreements_created_at_idx ON agreements (created_at DESC);

-- +goose Down

DROP TABLE IF EXISTS agreements;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
