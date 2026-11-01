CREATE TABLE IF NOT EXISTS trade_invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    invoice_code VARCHAR(16) UNIQUE NOT NULL,       -- e.g. 'trd_8f3a9b'
    importer_tag VARCHAR(32) NOT NULL,
    supplier_name VARCHAR(100) NOT NULL,            -- e.g. 'Shenzhen Top Display Co.'
    supplier_destination TEXT NOT NULL,             -- USDT address or Dubai/China bank
    amount_usd NUMERIC(12,2) NOT NULL,
    amount_units BIGINT NOT NULL,                   -- in micro-units
    amount_ngn NUMERIC(15,2) NOT NULL,
    fee_units BIGINT NOT NULL,                      -- 0.75% B2B trade take-rate
    narration TEXT NOT NULL,
    shipping_ref TEXT DEFAULT '',                   -- Bill of Lading / Airway Bill
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',  -- 'PENDING', 'IN_ESCROW', 'DISPATCHED', 'RELEASED', 'REFUNDED'
    funded_at TIMESTAMP WITH TIME ZONE,
    released_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_trade_code ON trade_invoices(invoice_code);
CREATE INDEX IF NOT EXISTS idx_trade_importer ON trade_invoices(importer_tag);
