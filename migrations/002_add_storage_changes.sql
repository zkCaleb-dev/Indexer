-- Migration 002: Add storage_changes table and event deduplication constraint

-- Table for tracking smart contract storage changes
CREATE TABLE IF NOT EXISTS storage_changes (
    id BIGSERIAL PRIMARY KEY,

    -- Identification
    contract_id VARCHAR(70) NOT NULL,
    change_type VARCHAR(20) NOT NULL, -- CREATED, UPDATED, REMOVED, RESTORED

    -- Storage key/value (parsed from SCVal)
    storage_key JSONB NOT NULL,        -- Parsed SCVal key
    storage_value JSONB,               -- Parsed SCVal value (NULL if removed)
    previous_value JSONB,              -- Previous value (NULL if created/restored)

    -- Raw XDR data (for exact reconstruction)
    raw_key BYTEA NOT NULL,
    raw_value BYTEA,
    raw_previous_value BYTEA,

    -- Durability type
    durability VARCHAR(20) NOT NULL,   -- TEMPORARY or PERSISTENT

    -- Transaction context
    tx_hash VARCHAR(64) NOT NULL,
    ledger_seq INTEGER NOT NULL,
    operation_index INTEGER,           -- Which operation caused the change (NULL if tx-level)
    timestamp TIMESTAMP NOT NULL,

    -- Metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for efficient querying
CREATE INDEX idx_storage_changes_contract_id ON storage_changes(contract_id);
CREATE INDEX idx_storage_changes_tx_hash ON storage_changes(tx_hash);
CREATE INDEX idx_storage_changes_ledger ON storage_changes(ledger_seq);
CREATE INDEX idx_storage_changes_timestamp ON storage_changes(timestamp);
CREATE INDEX idx_storage_changes_type ON storage_changes(change_type);
CREATE INDEX idx_storage_changes_key ON storage_changes USING GIN(storage_key);
CREATE INDEX idx_storage_changes_value ON storage_changes USING GIN(storage_value);
CREATE INDEX idx_storage_changes_contract_ledger ON storage_changes(contract_id, ledger_seq DESC);
CREATE INDEX idx_storage_changes_contract_type ON storage_changes(contract_id, change_type);

-- Add unique constraint to contract_events to prevent duplicate events
ALTER TABLE contract_events
ADD CONSTRAINT unique_contract_event
UNIQUE (tx_hash, event_index, contract_id);

-- Comments for documentation
COMMENT ON TABLE storage_changes IS 'Tracks all storage state changes for Soroban smart contracts';
COMMENT ON COLUMN storage_changes.change_type IS 'Type of change: CREATED, UPDATED, REMOVED, or RESTORED';
COMMENT ON COLUMN storage_changes.durability IS 'Storage durability: TEMPORARY or PERSISTENT';
COMMENT ON COLUMN storage_changes.storage_key IS 'Parsed SCVal storage key as JSON';
COMMENT ON COLUMN storage_changes.storage_value IS 'Parsed SCVal storage value as JSON (NULL if removed)';
COMMENT ON COLUMN storage_changes.previous_value IS 'Previous value for UPDATED/REMOVED changes (NULL for CREATED/RESTORED)';
