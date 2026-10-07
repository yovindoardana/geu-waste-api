CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY,
    household_id UUID NOT NULL,
    waste_id UUID NOT NULL,
    amount NUMERIC(12,2) NOT NULL,
    payment_date TIMESTAMPTZ NULL DEFAULT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    proof_file_url VARCHAR(500) NULL DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT payments_household_id_fkey FOREIGN KEY (household_id) REFERENCES households(id) ON DELETE RESTRICT,
    CONSTRAINT payments_waste_household_fkey FOREIGN KEY (waste_id, household_id) REFERENCES waste_pickups(id, household_id) ON DELETE RESTRICT,
    CONSTRAINT payments_waste_id_key UNIQUE (waste_id),
    CONSTRAINT payments_amount_check CHECK (amount > 0 AND amount <= 9999999999.99),
    CONSTRAINT payments_status_check CHECK (status IN ('pending', 'paid', 'failed')),
    CONSTRAINT payments_confirmation_check CHECK (
        (status = 'paid'
         AND payment_date IS NOT NULL
         AND proof_file_url IS NOT NULL
         AND char_length(btrim(proof_file_url)) > 0)
        OR
        (status IN ('pending', 'failed')
         AND payment_date IS NULL
         AND proof_file_url IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_payments_created_at_id ON payments (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_payments_household_created_at_id ON payments (household_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_payments_status_created_at_id ON payments (status, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_payments_pending_household ON payments (household_id) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_payments_payment_date ON payments (payment_date) WHERE payment_date IS NOT NULL;
