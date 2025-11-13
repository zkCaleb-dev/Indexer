-- Initial schema for Stellar Indexer
-- Created: 2025-11-13

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =====================================================
-- DEPLOYED CONTRACTS TABLE
-- =====================================================
CREATE TABLE IF NOT EXISTS deployed_contracts (
    id SERIAL PRIMARY KEY,

    -- Identification
    contract_id VARCHAR(56) NOT NULL UNIQUE,
    factory_contract_id VARCHAR(56) NOT NULL,

    -- Deployment metadata
    deployed_at_ledger INTEGER NOT NULL,
    deployed_at_time TIMESTAMP NOT NULL,
    tx_hash VARCHAR(64) NOT NULL,
    deployer VARCHAR(56) NOT NULL,

    -- Costs and resources
    fee_charged BIGINT NOT NULL,
    cpu_instructions BIGINT NOT NULL DEFAULT 0,
    memory_bytes INTEGER NOT NULL DEFAULT 0,

    -- Initialization data (JSONB for flexible storage)
    init_params JSONB,

    -- Metadata
    memo TEXT,
    memo_type VARCHAR(20),

    -- Timestamps
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for deployed_contracts
CREATE INDEX idx_deployed_contracts_contract_id ON deployed_contracts(contract_id);
CREATE INDEX idx_deployed_contracts_factory ON deployed_contracts(factory_contract_id);
CREATE INDEX idx_deployed_contracts_ledger ON deployed_contracts(deployed_at_ledger);
CREATE INDEX idx_deployed_contracts_time ON deployed_contracts(deployed_at_time);
CREATE INDEX idx_deployed_contracts_deployer ON deployed_contracts(deployer);
CREATE INDEX idx_deployed_contracts_tx_hash ON deployed_contracts(tx_hash);

-- GIN index for JSONB queries on init_params
CREATE INDEX idx_deployed_contracts_init_params ON deployed_contracts USING GIN (init_params);

-- =====================================================
-- CONTRACT EVENTS TABLE
-- =====================================================
CREATE TABLE IF NOT EXISTS contract_events (
    id SERIAL PRIMARY KEY,

    -- Event identification
    contract_id VARCHAR(56) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_index INTEGER NOT NULL,

    -- Event data
    topics TEXT[] NOT NULL,
    data JSONB NOT NULL,
    raw_topics BYTEA[],
    raw_data BYTEA,

    -- Transaction context
    tx_hash VARCHAR(64) NOT NULL,
    ledger_seq INTEGER NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    in_successful_contract_call BOOLEAN DEFAULT true,

    -- Timestamps
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for contract_events
CREATE INDEX idx_contract_events_contract_id ON contract_events(contract_id);
CREATE INDEX idx_contract_events_type ON contract_events(event_type);
CREATE INDEX idx_contract_events_tx_hash ON contract_events(tx_hash);
CREATE INDEX idx_contract_events_ledger ON contract_events(ledger_seq);
CREATE INDEX idx_contract_events_timestamp ON contract_events(timestamp);

-- GIN index for JSONB queries on event data
CREATE INDEX idx_contract_events_data ON contract_events USING GIN (data);

-- Composite index for common queries
CREATE INDEX idx_contract_events_contract_ledger ON contract_events(contract_id, ledger_seq DESC);

-- =====================================================
-- STORAGE ENTRIES TABLE
-- =====================================================
CREATE TABLE IF NOT EXISTS storage_entries (
    id SERIAL PRIMARY KEY,

    -- Storage identification
    contract_id VARCHAR(56) NOT NULL,
    key VARCHAR(255) NOT NULL,
    key_type VARCHAR(50) NOT NULL,

    -- Storage data
    value JSONB,
    value_type VARCHAR(50) NOT NULL,
    raw_key BYTEA,
    raw_value BYTEA,

    -- Change tracking
    change_type VARCHAR(20) NOT NULL, -- 'created', 'updated', 'removed'
    previous_value JSONB,

    -- Transaction context
    tx_hash VARCHAR(64) NOT NULL,
    ledger_seq INTEGER NOT NULL,

    -- Timestamps
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for storage_entries
CREATE INDEX idx_storage_entries_contract_id ON storage_entries(contract_id);
CREATE INDEX idx_storage_entries_key ON storage_entries(key);
CREATE INDEX idx_storage_entries_ledger ON storage_entries(ledger_seq);
CREATE INDEX idx_storage_entries_change_type ON storage_entries(change_type);

-- Composite index for latest state queries
CREATE INDEX idx_storage_entries_contract_key ON storage_entries(contract_id, key, ledger_seq DESC);

-- GIN indexes for JSONB queries
CREATE INDEX idx_storage_entries_value ON storage_entries USING GIN (value);
CREATE INDEX idx_storage_entries_previous_value ON storage_entries USING GIN (previous_value);

-- =====================================================
-- CONTRACT ACTIVITIES TABLE
-- =====================================================
CREATE TABLE IF NOT EXISTS contract_activities (
    id SERIAL PRIMARY KEY,

    -- Activity identification
    activity_id VARCHAR(100) NOT NULL UNIQUE,
    contract_id VARCHAR(56) NOT NULL,
    activity_type VARCHAR(50) NOT NULL, -- 'invocation', 'deployment', etc.

    -- Transaction context
    tx_hash VARCHAR(64) NOT NULL,
    ledger_seq INTEGER NOT NULL,
    timestamp TIMESTAMP NOT NULL,

    -- Invocation details
    invoker VARCHAR(56),
    function_name VARCHAR(100),
    parameters JSONB,

    -- Result
    success BOOLEAN NOT NULL,
    return_value JSONB,
    error_message TEXT,

    -- Resources
    fee_charged BIGINT NOT NULL DEFAULT 0,
    cpu_instructions BIGINT NOT NULL DEFAULT 0,
    memory_bytes INTEGER NOT NULL DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for contract_activities
CREATE INDEX idx_contract_activities_activity_id ON contract_activities(activity_id);
CREATE INDEX idx_contract_activities_contract_id ON contract_activities(contract_id);
CREATE INDEX idx_contract_activities_type ON contract_activities(activity_type);
CREATE INDEX idx_contract_activities_tx_hash ON contract_activities(tx_hash);
CREATE INDEX idx_contract_activities_ledger ON contract_activities(ledger_seq);
CREATE INDEX idx_contract_activities_timestamp ON contract_activities(timestamp);
CREATE INDEX idx_contract_activities_invoker ON contract_activities(invoker);
CREATE INDEX idx_contract_activities_success ON contract_activities(success);

-- GIN indexes for JSONB queries
CREATE INDEX idx_contract_activities_parameters ON contract_activities USING GIN (parameters);
CREATE INDEX idx_contract_activities_return_value ON contract_activities USING GIN (return_value);

-- Composite index for common queries
CREATE INDEX idx_contract_activities_contract_time ON contract_activities(contract_id, timestamp DESC);

-- =====================================================
-- LEDGER INFO TABLE (for tracking processed ledgers)
-- =====================================================
CREATE TABLE IF NOT EXISTS ledger_info (
    id SERIAL PRIMARY KEY,

    -- Ledger identification
    sequence INTEGER NOT NULL UNIQUE,

    -- Ledger metadata
    closed_at TIMESTAMP NOT NULL,
    tx_count INTEGER NOT NULL DEFAULT 0,
    soroban_tx_count INTEGER NOT NULL DEFAULT 0,
    successful_tx_count INTEGER NOT NULL DEFAULT 0,
    failed_tx_count INTEGER NOT NULL DEFAULT 0,

    -- Processing metadata
    processed_at TIMESTAMP DEFAULT NOW(),
    processing_time_ms INTEGER DEFAULT 0
);

-- Indexes for ledger_info
CREATE INDEX idx_ledger_info_sequence ON ledger_info(sequence);
CREATE INDEX idx_ledger_info_closed_at ON ledger_info(closed_at);

-- =====================================================
-- RELATIONS AND FOREIGN KEYS
-- =====================================================

-- Link events to deployed contracts (optional, for referential integrity)
-- Commented out because events can exist for contracts not in our tracking
-- ALTER TABLE contract_events
--     ADD CONSTRAINT fk_contract_events_contract
--     FOREIGN KEY (contract_id)
--     REFERENCES deployed_contracts(contract_id)
--     ON DELETE CASCADE;

-- =====================================================
-- FUNCTIONS AND TRIGGERS
-- =====================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger for deployed_contracts
CREATE TRIGGER update_deployed_contracts_updated_at
    BEFORE UPDATE ON deployed_contracts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =====================================================
-- VIEWS FOR COMMON QUERIES
-- =====================================================

-- View for latest storage state per contract
CREATE OR REPLACE VIEW latest_storage_state AS
SELECT DISTINCT ON (contract_id, key)
    contract_id,
    key,
    value,
    value_type,
    ledger_seq,
    created_at
FROM storage_entries
WHERE change_type != 'removed'
ORDER BY contract_id, key, ledger_seq DESC;

-- View for contract statistics
CREATE OR REPLACE VIEW contract_statistics AS
SELECT
    dc.contract_id,
    dc.factory_contract_id,
    dc.deployed_at_ledger,
    dc.deployed_at_time,
    COUNT(DISTINCT ca.id) as total_invocations,
    COUNT(DISTINCT ce.id) as total_events,
    COUNT(DISTINCT se.id) as total_storage_changes,
    SUM(ca.fee_charged) as total_fees_paid
FROM deployed_contracts dc
LEFT JOIN contract_activities ca ON dc.contract_id = ca.contract_id
LEFT JOIN contract_events ce ON dc.contract_id = ce.contract_id
LEFT JOIN storage_entries se ON dc.contract_id = se.contract_id
GROUP BY dc.contract_id, dc.factory_contract_id, dc.deployed_at_ledger, dc.deployed_at_time;

-- =====================================================
-- COMMENTS FOR DOCUMENTATION
-- =====================================================

COMMENT ON TABLE deployed_contracts IS 'Contracts deployed through tracked factory contracts';
COMMENT ON TABLE contract_events IS 'Events emitted by tracked contracts';
COMMENT ON TABLE storage_entries IS 'Storage changes for tracked contracts';
COMMENT ON TABLE contract_activities IS 'All interactions with tracked contracts';
COMMENT ON TABLE ledger_info IS 'Metadata about processed ledgers';

COMMENT ON COLUMN deployed_contracts.init_params IS 'Initialization parameters extracted from deployment return value';
COMMENT ON COLUMN contract_events.data IS 'Parsed event data as JSONB';
COMMENT ON COLUMN storage_entries.change_type IS 'Type of change: created, updated, or removed';
COMMENT ON COLUMN contract_activities.activity_type IS 'Type of activity: invocation, deployment, etc.';
