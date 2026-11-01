-- Add auth credentials and progressive KYC fields to identities
ALTER TABLE identities 
ADD COLUMN IF NOT EXISTS pin_hash VARCHAR(128) NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS raw_phone VARCHAR(20) DEFAULT '',
ADD COLUMN IF NOT EXISTS full_name VARCHAR(100) DEFAULT '',
ADD COLUMN IF NOT EXISTS kyc_tier VARCHAR(32) NOT NULL DEFAULT 'TIER_0_STARTER',
ADD COLUMN IF NOT EXISTS kyc_data JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_identities_raw_phone ON identities(raw_phone);
