-- Migration: Fix durability column length to accommodate full enum values
-- ContractDataDurabilityPersistent = 32 characters, we use 40 for safety

BEGIN;

-- Update storage_changes table
ALTER TABLE storage_changes
    ALTER COLUMN durability TYPE VARCHAR(40);

COMMIT;
