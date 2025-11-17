-- Migration 007: Add contract_type column to deployed_contracts table

-- Add contract_type column (nullable for backward compatibility)
ALTER TABLE deployed_contracts
ADD COLUMN contract_type VARCHAR(20);

-- Create index for efficient filtering by contract type
CREATE INDEX idx_deployed_contracts_type ON deployed_contracts(contract_type);

-- Update existing rows to have a default value (assuming they are single-release)
UPDATE deployed_contracts
SET contract_type = 'single-release'
WHERE contract_type IS NULL;

-- Comments for documentation
COMMENT ON COLUMN deployed_contracts.contract_type
IS 'Type of factory that deployed this contract: single-release or multi-release';
