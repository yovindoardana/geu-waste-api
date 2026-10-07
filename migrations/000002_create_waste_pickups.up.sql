CREATE TABLE IF NOT EXISTS waste_pickups (
    id UUID PRIMARY KEY,
    household_id UUID NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    pickup_date TIMESTAMPTZ NULL DEFAULT NULL,
    safety_check BOOLEAN NULL DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT waste_pickups_household_id_fkey FOREIGN KEY (household_id) REFERENCES households(id) ON DELETE RESTRICT,
    CONSTRAINT waste_pickups_id_household_id_key UNIQUE (id, household_id),
    CONSTRAINT waste_pickups_type_check CHECK (type IN ('organic', 'plastic', 'paper', 'electronic')),
    CONSTRAINT waste_pickups_status_check CHECK (status IN ('pending', 'scheduled', 'completed', 'canceled')),
    CONSTRAINT waste_pickups_safety_presence_check CHECK (
        (type = 'electronic' AND safety_check IS NOT NULL)
        OR (type <> 'electronic' AND safety_check IS NULL)
    ),
    CONSTRAINT waste_pickups_scheduled_date_check CHECK (
        status NOT IN ('scheduled', 'completed') OR pickup_date IS NOT NULL
    ),
    CONSTRAINT waste_pickups_pending_date_check CHECK (
        status <> 'pending' OR pickup_date IS NULL
    ),
    CONSTRAINT waste_pickups_electronic_safety_check CHECK (
        type <> 'electronic'
        OR status NOT IN ('scheduled', 'completed')
        OR safety_check IS TRUE
    )
);

CREATE INDEX IF NOT EXISTS idx_waste_pickups_created_at_id ON waste_pickups (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_waste_pickups_household_created_at_id ON waste_pickups (household_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_waste_pickups_status_created_at_id ON waste_pickups (status, created_at DESC, id DESC);
