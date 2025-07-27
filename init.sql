CREATE UNLOGGED TABLE IF NOT EXISTS payments (
    correlation_id UUID PRIMARY KEY,
    amount DECIMAL(10, 2) NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL,
    processor SMALLINT NOT NULL
);

CREATE INDEX CONCURRENTLY idx_processor_requested_at ON payments(requested_at, processor);