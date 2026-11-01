-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Accounts Table: Holds balances for Users, Merchants, Treasuries, and Escrows
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    holder_id UUID NOT NULL,
    corridor VARCHAR(4) NOT NULL, -- 'NG', 'KE', 'GH', 'TZ', 'GLOBAL'
    currency VARCHAR(12) NOT NULL DEFAULT 'USDC',
    account_type VARCHAR(32) NOT NULL, -- 'ASSET', 'LIABILITY', 'EQUITY', 'REVENUE', 'EXPENSE'
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_holder ON accounts(holder_id);
CREATE INDEX idx_accounts_corridor ON accounts(corridor);

-- 2. Journal Entries: Immutable record of an economic event
CREATE TABLE journal_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    idempotency_key VARCHAR(128) UNIQUE NOT NULL,
    reference_type VARCHAR(64) NOT NULL, -- 'POS_CHECKOUT', 'P2P_TRANSFER', 'VAS_AIRTIME', 'ARC_SPLIT'
    narration TEXT NOT NULL,
    corridor VARCHAR(4) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_journal_idempotency ON journal_entries(idempotency_key);

-- 3. Postings: Atomic Debits and Credits in micro-units (1 USDC = 1,000,000 units)
-- Sum of Debits MUST equal Sum of Credits per journal entry
CREATE TABLE postings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    journal_entry_id UUID NOT NULL REFERENCES journal_entries(id) ON DELETE RESTRICT,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    amount_units BIGINT NOT NULL CHECK (amount_units > 0),
    direction VARCHAR(6) NOT NULL CHECK (direction IN ('DEBIT', 'CREDIT')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_postings_journal ON postings(journal_entry_id);
CREATE INDEX idx_postings_account ON postings(account_id);