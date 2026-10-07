CREATE TABLE IF NOT EXISTS households (
    id UUID PRIMARY KEY,
    owner_name VARCHAR(150) NOT NULL,
    address VARCHAR(1000) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT households_owner_name_check CHECK (char_length(btrim(owner_name)) > 0),
    CONSTRAINT households_address_check CHECK (char_length(btrim(address)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_households_created_at_id ON households (created_at DESC, id DESC);
