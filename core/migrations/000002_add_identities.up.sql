CREATE TABLE IF NOT EXISTS identities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    arc_tag VARCHAR(32) UNIQUE NOT NULL,
    phone_hash VARCHAR(64) UNIQUE,
    holder_id UUID NOT NULL,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    corridor VARCHAR(4) NOT NULL,
    evm_address VARCHAR(42),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_identities_tag ON identities(arc_tag);
CREATE INDEX IF NOT EXISTS idx_identities_phone ON identities(phone_hash);
CREATE INDEX IF NOT EXISTS idx_identities_account ON identities(account_id);
