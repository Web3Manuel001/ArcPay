CREATE TABLE IF NOT EXISTS split_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    split_code VARCHAR(16) UNIQUE NOT NULL,      -- e.g., 'spl_8f3a9b'
    creator_tag VARCHAR(32) NOT NULL,
    total_amount_units BIGINT NOT NULL,          -- in micro-units
    share_per_person_units BIGINT NOT NULL,
    currency VARCHAR(12) NOT NULL DEFAULT 'USDC',
    narration TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN', -- 'OPEN', 'COMPLETED', 'EXPIRED'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS split_participants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    split_id UUID NOT NULL REFERENCES split_sessions(id) ON DELETE CASCADE,
    participant_tag VARCHAR(32),                 -- Optional (if already an Arcpay user)
    phone_or_email VARCHAR(64) NOT NULL,
    amount_units BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- 'PENDING', 'PAID'
    payment_method VARCHAR(20),                  -- 'ARCPAY_BALANCE', 'CARD', 'BANK_TRANSFER'
    paid_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_split_code ON split_sessions(split_code);
CREATE INDEX IF NOT EXISTS idx_split_participants ON split_participants(split_id);
