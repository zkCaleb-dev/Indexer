-- Migration 006: Add indexer progress tracking table

-- Table for tracking indexer progress (checkpoint/resume functionality)
CREATE TABLE IF NOT EXISTS indexer_progress (
    id INTEGER PRIMARY KEY DEFAULT 1,
    last_ledger_processed INTEGER NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Ensure only one row exists
    CONSTRAINT single_row CHECK (id = 1)
);

-- Index for quick lookups (though there's only 1 row)
CREATE INDEX idx_indexer_progress_ledger ON indexer_progress(last_ledger_processed);

-- Comments for documentation
COMMENT ON TABLE indexer_progress IS 'Tracks the last successfully processed ledger for checkpoint/resume functionality';
COMMENT ON COLUMN indexer_progress.last_ledger_processed IS 'Ledger sequence number of the last successfully processed ledger';
COMMENT ON COLUMN indexer_progress.updated_at IS 'Timestamp when the progress was last updated';

-- Trigger to update updated_at on each update
CREATE TRIGGER update_indexer_progress_timestamp
    BEFORE UPDATE ON indexer_progress
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
