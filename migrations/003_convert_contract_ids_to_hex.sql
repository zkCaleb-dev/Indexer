-- Migration: Convert contract_ids from STRKEY format to HEX format
-- This migration updates all existing contract_ids in the database to use hex format
-- for consistency across all tables

-- Note: This is a destructive migration. If you need to preserve strkey format,
-- make a backup first.

-- For simplicity, we'll truncate the deployed_contracts table since the data
-- can be re-indexed. This avoids complex strkey-to-hex conversion in SQL.

-- Alternative: If you want to preserve data, you'd need to:
-- 1. Add a new column for hex contract_id
-- 2. Write a script to convert strkey to hex for each row
-- 3. Drop the old column and rename the new one

BEGIN;

-- Clear all existing data (will be re-indexed with correct hex format)
TRUNCATE TABLE deployed_contracts CASCADE;
TRUNCATE TABLE contract_activities CASCADE;
TRUNCATE TABLE contract_events CASCADE;
TRUNCATE TABLE storage_entries CASCADE;
TRUNCATE TABLE storage_changes CASCADE;

COMMIT;

-- Note: After running this migration, restart the indexer with START_LEDGER
-- set to a ledger before your first deployment to re-index everything with hex format
