-- Migration: Update contract_id column length from 56 to 70 characters
-- HEX format requires 64 characters, we use 70 for safety

BEGIN;

-- Drop views that depend on contract_id columns
DROP VIEW IF EXISTS contract_statistics CASCADE;
DROP VIEW IF EXISTS latest_storage_state CASCADE;

-- Update deployed_contracts table
ALTER TABLE deployed_contracts
    ALTER COLUMN contract_id TYPE VARCHAR(70),
    ALTER COLUMN factory_contract_id TYPE VARCHAR(70);

-- Update contract_activities table
ALTER TABLE contract_activities
    ALTER COLUMN contract_id TYPE VARCHAR(70);

-- Update contract_events table
ALTER TABLE contract_events
    ALTER COLUMN contract_id TYPE VARCHAR(70);

-- Update storage_entries table
ALTER TABLE storage_entries
    ALTER COLUMN contract_id TYPE VARCHAR(70);

-- Update storage_changes table
ALTER TABLE storage_changes
    ALTER COLUMN contract_id TYPE VARCHAR(70);

-- Recreate contract_statistics view
CREATE OR REPLACE VIEW contract_statistics AS
SELECT
    dc.contract_id,
    dc.factory_contract_id,
    dc.deployer,
    COUNT(DISTINCT ce.id) AS event_count,
    COUNT(DISTINCT se.id) AS storage_entry_count,
    COUNT(DISTINCT ca.id) AS activity_count,
    dc.deployed_at_time AS deployed_at
FROM deployed_contracts dc
LEFT JOIN contract_events ce ON dc.contract_id = ce.contract_id
LEFT JOIN storage_entries se ON dc.contract_id = se.contract_id
LEFT JOIN contract_activities ca ON dc.contract_id = ca.contract_id
GROUP BY dc.contract_id, dc.factory_contract_id, dc.deployer, dc.deployed_at_time;

-- Recreate latest_storage_state view
CREATE OR REPLACE VIEW latest_storage_state AS
SELECT DISTINCT ON (contract_id, key)
    contract_id,
    key,
    value,
    change_type,
    ledger_seq,
    tx_hash,
    created_at
FROM storage_entries
ORDER BY contract_id, key, ledger_seq DESC;

COMMIT;
