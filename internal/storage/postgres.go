package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"indexer/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements the Repository interface using PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresRepository{
		pool: pool,
	}, nil
}

// SaveDeployedContract saves a deployed contract to the database
func (r *PostgresRepository) SaveDeployedContract(ctx context.Context, contract *models.DeployedContract) error {
	initParamsJSON, err := json.Marshal(contract.InitParams)
	if err != nil {
		return fmt.Errorf("failed to marshal init_params: %w", err)
	}

	query := `
		INSERT INTO deployed_contracts (
			contract_id, factory_contract_id, deployed_at_ledger, deployed_at_time,
			tx_hash, deployer, fee_charged, cpu_instructions, memory_bytes,
			init_params, memo, memo_type
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (contract_id) DO NOTHING
	`

	_, err = r.pool.Exec(ctx, query,
		contract.ContractID,
		contract.FactoryContractID,
		contract.DeployedAtLedger,
		contract.DeployedAtTime,
		contract.TxHash,
		contract.Deployer,
		contract.FeeCharged,
		contract.CPUInstructions,
		contract.MemoryBytes,
		initParamsJSON,
		contract.Memo,
		contract.MemoType,
	)

	if err != nil {
		return fmt.Errorf("failed to save deployed contract: %w", err)
	}

	return nil
}

// GetDeployedContract retrieves a deployed contract by contract ID
func (r *PostgresRepository) GetDeployedContract(ctx context.Context, contractID string) (*models.DeployedContract, error) {
	query := `
		SELECT
			contract_id, factory_contract_id, deployed_at_ledger, deployed_at_time,
			tx_hash, deployer, fee_charged, cpu_instructions, memory_bytes,
			init_params, memo, memo_type
		FROM deployed_contracts
		WHERE contract_id = $1
	`

	var contract models.DeployedContract
	var initParamsJSON []byte

	err := r.pool.QueryRow(ctx, query, contractID).Scan(
		&contract.ContractID,
		&contract.FactoryContractID,
		&contract.DeployedAtLedger,
		&contract.DeployedAtTime,
		&contract.TxHash,
		&contract.Deployer,
		&contract.FeeCharged,
		&contract.CPUInstructions,
		&contract.MemoryBytes,
		&initParamsJSON,
		&contract.Memo,
		&contract.MemoType,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("contract not found: %s", contractID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get deployed contract: %w", err)
	}

	if err := json.Unmarshal(initParamsJSON, &contract.InitParams); err != nil {
		return nil, fmt.Errorf("failed to unmarshal init_params: %w", err)
	}

	return &contract, nil
}

// ListDeployedContracts lists all deployed contracts with pagination
func (r *PostgresRepository) ListDeployedContracts(ctx context.Context, limit, offset int) ([]*models.DeployedContract, error) {
	query := `
		SELECT
			contract_id, factory_contract_id, deployed_at_ledger, deployed_at_time,
			tx_hash, deployer, fee_charged, cpu_instructions, memory_bytes,
			init_params, memo, memo_type
		FROM deployed_contracts
		ORDER BY deployed_at_ledger DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list deployed contracts: %w", err)
	}
	defer rows.Close()

	var contracts []*models.DeployedContract

	for rows.Next() {
		var contract models.DeployedContract
		var initParamsJSON []byte

		err := rows.Scan(
			&contract.ContractID,
			&contract.FactoryContractID,
			&contract.DeployedAtLedger,
			&contract.DeployedAtTime,
			&contract.TxHash,
			&contract.Deployer,
			&contract.FeeCharged,
			&contract.CPUInstructions,
			&contract.MemoryBytes,
			&initParamsJSON,
			&contract.Memo,
			&contract.MemoType,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan contract: %w", err)
		}

		if err := json.Unmarshal(initParamsJSON, &contract.InitParams); err != nil {
			return nil, fmt.Errorf("failed to unmarshal init_params: %w", err)
		}

		contracts = append(contracts, &contract)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating contracts: %w", err)
	}

	return contracts, nil
}

// GetTrackedContractIDs returns all contract IDs that are being tracked
func (r *PostgresRepository) GetTrackedContractIDs(ctx context.Context) ([]string, error) {
	query := `SELECT contract_id FROM deployed_contracts ORDER BY deployed_at_ledger ASC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get tracked contract IDs: %w", err)
	}
	defer rows.Close()

	var contractIDs []string
	for rows.Next() {
		var contractID string
		if err := rows.Scan(&contractID); err != nil {
			return nil, fmt.Errorf("failed to scan contract ID: %w", err)
		}
		contractIDs = append(contractIDs, contractID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating contract IDs: %w", err)
	}

	return contractIDs, nil
}

// SaveContractEvent saves a single contract event
func (r *PostgresRepository) SaveContractEvent(ctx context.Context, event *models.ContractEvent) error {
	dataJSON, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	query := `
		INSERT INTO contract_events (
			contract_id, event_type, event_index, topics, data,
			raw_data, tx_hash, ledger_seq, timestamp, in_successful_contract_call
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.pool.Exec(ctx, query,
		event.ContractID,
		event.EventType,
		event.EventIndex,
		event.Topics,
		dataJSON,
		event.RawData,
		event.TxHash,
		event.LedgerSeq,
		event.Timestamp,
		event.InSuccessfulContractCall,
	)

	if err != nil {
		return fmt.Errorf("failed to save contract event: %w", err)
	}

	return nil
}

// SaveContractEvents saves multiple contract events in a transaction
func (r *PostgresRepository) SaveContractEvents(ctx context.Context, events []models.ContractEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO contract_events (
			contract_id, event_type, event_index, topics, data,
			raw_data, tx_hash, ledger_seq, timestamp, in_successful_contract_call
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	for _, event := range events {
		dataJSON, err := json.Marshal(event.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}

		_, err = tx.Exec(ctx, query,
			event.ContractID,
			event.EventType,
			event.EventIndex,
			event.Topics,
			dataJSON,
			event.RawData,
			event.TxHash,
			event.LedgerSeq,
			event.Timestamp,
			event.InSuccessfulContractCall,
		)

		if err != nil {
			return fmt.Errorf("failed to save event: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ListContractEvents lists events for a specific contract with pagination
func (r *PostgresRepository) ListContractEvents(ctx context.Context, contractID string, limit, offset int) ([]models.ContractEvent, error) {
	query := `
		SELECT
			contract_id, event_type, event_index, topics, data,
			raw_data, tx_hash, ledger_seq, timestamp, in_successful_contract_call
		FROM contract_events
		WHERE contract_id = $1
		ORDER BY ledger_seq DESC, event_index ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, contractID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list contract events: %w", err)
	}
	defer rows.Close()

	var events []models.ContractEvent

	for rows.Next() {
		var event models.ContractEvent
		var dataJSON []byte

		err := rows.Scan(
			&event.ContractID,
			&event.EventType,
			&event.EventIndex,
			&event.Topics,
			&dataJSON,
			&event.RawData,
			&event.TxHash,
			&event.LedgerSeq,
			&event.Timestamp,
			&event.InSuccessfulContractCall,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if err := json.Unmarshal(dataJSON, &event.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	return events, nil
}

// SaveStorageEntry saves a single storage entry
func (r *PostgresRepository) SaveStorageEntry(ctx context.Context, entry *models.StorageEntry) error {
	valueJSON, err := json.Marshal(entry.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	previousValueJSON, err := json.Marshal(entry.PreviousValue)
	if err != nil {
		return fmt.Errorf("failed to marshal previous_value: %w", err)
	}

	query := `
		INSERT INTO storage_entries (
			contract_id, key, key_type, value, value_type,
			raw_key, raw_value, change_type, previous_value,
			tx_hash, ledger_seq
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err = r.pool.Exec(ctx, query,
		entry.ContractID,
		entry.Key,
		entry.KeyType,
		valueJSON,
		entry.ValueType,
		entry.RawKey,
		entry.RawValue,
		entry.ChangeType,
		previousValueJSON,
		entry.TxHash,
		entry.LedgerSeq,
	)

	if err != nil {
		return fmt.Errorf("failed to save storage entry: %w", err)
	}

	return nil
}

// SaveStorageEntries saves multiple storage entries in a transaction
func (r *PostgresRepository) SaveStorageEntries(ctx context.Context, entries []models.StorageEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO storage_entries (
			contract_id, key, key_type, value, value_type,
			raw_key, raw_value, change_type, previous_value,
			tx_hash, ledger_seq
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	for _, entry := range entries {
		valueJSON, err := json.Marshal(entry.Value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}

		previousValueJSON, err := json.Marshal(entry.PreviousValue)
		if err != nil {
			return fmt.Errorf("failed to marshal previous_value: %w", err)
		}

		_, err = tx.Exec(ctx, query,
			entry.ContractID,
			entry.Key,
			entry.KeyType,
			valueJSON,
			entry.ValueType,
			entry.RawKey,
			entry.RawValue,
			entry.ChangeType,
			previousValueJSON,
			entry.TxHash,
			entry.LedgerSeq,
		)

		if err != nil {
			return fmt.Errorf("failed to save storage entry: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetLatestStorageState retrieves the latest storage state for a contract
func (r *PostgresRepository) GetLatestStorageState(ctx context.Context, contractID string) ([]models.StorageEntry, error) {
	query := `
		SELECT contract_id, key, value, value_type, ledger_seq
		FROM latest_storage_state
		WHERE contract_id = $1
		ORDER BY key ASC
	`

	rows, err := r.pool.Query(ctx, query, contractID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest storage state: %w", err)
	}
	defer rows.Close()

	var entries []models.StorageEntry

	for rows.Next() {
		var entry models.StorageEntry
		var valueJSON []byte

		err := rows.Scan(
			&entry.ContractID,
			&entry.Key,
			&valueJSON,
			&entry.ValueType,
			&entry.LedgerSeq,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan storage entry: %w", err)
		}

		if err := json.Unmarshal(valueJSON, &entry.Value); err != nil {
			return nil, fmt.Errorf("failed to unmarshal value: %w", err)
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating storage entries: %w", err)
	}

	return entries, nil
}

// SaveContractActivity saves a contract activity
func (r *PostgresRepository) SaveContractActivity(ctx context.Context, activity *models.ContractActivity) error {
	parametersJSON, err := json.Marshal(activity.Parameters)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}
	returnValueJSON, err2 := json.Marshal(activity.ReturnValue)
	if err2 != nil {
		return fmt.Errorf("failed to marshal return value: %w", err2)
	}

	query := `
		INSERT INTO contract_activities (
			activity_id, contract_id, activity_type, tx_hash, ledger_seq, timestamp,
			invoker, function_name, parameters, success, return_value, error_message,
			fee_charged, cpu_instructions, memory_bytes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (activity_id) DO NOTHING
	`

	_, err = r.pool.Exec(ctx, query,
		activity.ActivityID,
		activity.ContractID,
		activity.ActivityType,
		activity.TxHash,
		activity.LedgerSeq,
		activity.Timestamp,
		activity.Invoker,
		activity.FunctionName,
		parametersJSON,
		activity.Success,
		returnValueJSON,
		activity.FailureReason,
		activity.FeeCharged,
		activity.CPUInstructions,
		activity.MemoryBytes,
	)

	if err != nil {
		return fmt.Errorf("failed to save contract activity: %w", err)
	}

	return nil
}

// ListContractActivities lists activities for a specific contract with pagination
func (r *PostgresRepository) ListContractActivities(ctx context.Context, contractID string, limit, offset int) ([]*models.ContractActivity, error) {
	query := `
		SELECT
			activity_id, contract_id, activity_type, tx_hash, ledger_seq, timestamp,
			invoker, function_name, parameters, success, return_value, error_message,
			fee_charged, cpu_instructions, memory_bytes
		FROM contract_activities
		WHERE contract_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, contractID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list contract activities: %w", err)
	}
	defer rows.Close()

	var activities []*models.ContractActivity

	for rows.Next() {
		var activity models.ContractActivity
		var parametersJSON, returnValueJSON []byte

		err := rows.Scan(
			&activity.ActivityID,
			&activity.ContractID,
			&activity.ActivityType,
			&activity.TxHash,
			&activity.LedgerSeq,
			&activity.Timestamp,
			&activity.Invoker,
			&activity.FunctionName,
			&parametersJSON,
			&activity.Success,
			&returnValueJSON,
			&activity.FailureReason,
			&activity.FeeCharged,
			&activity.CPUInstructions,
			&activity.MemoryBytes,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan activity: %w", err)
		}

		if err := json.Unmarshal(parametersJSON, &activity.Parameters); err != nil {
			slog.Warn("Failed to unmarshal parameters", "activity_id", activity.ActivityID, "error", err)
		}
		if err := json.Unmarshal(returnValueJSON, &activity.ReturnValue); err != nil {
			slog.Warn("Failed to unmarshal return value", "activity_id", activity.ActivityID, "error", err)
		}

		activities = append(activities, &activity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating activities: %w", err)
	}

	return activities, nil
}

// SaveLedgerInfo saves ledger processing information
func (r *PostgresRepository) SaveLedgerInfo(ctx context.Context, info *models.LedgerInfo) error {
	// Note: Schema has more fields than model currently provides
	// We'll save what we have, leaving other fields as default/0
	query := `
		INSERT INTO ledger_info (
			sequence, closed_at, tx_count, soroban_tx_count, processing_time_ms
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (sequence) DO NOTHING
	`

	_, err := r.pool.Exec(ctx, query,
		info.Sequence,
		info.CloseTime,
		info.TxCount,
		info.SorobanTxCount,
		info.ProcessingDuration,
	)

	if err != nil {
		return fmt.Errorf("failed to save ledger info: %w", err)
	}

	return nil
}

// GetLastProcessedLedger returns the sequence number of the last processed ledger
func (r *PostgresRepository) GetLastProcessedLedger(ctx context.Context) (uint32, error) {
	query := `SELECT COALESCE(MAX(sequence), 0) FROM ledger_info`

	var sequence uint32
	err := r.pool.QueryRow(ctx, query).Scan(&sequence)
	if err != nil {
		return 0, fmt.Errorf("failed to get last processed ledger: %w", err)
	}

	return sequence, nil
}

// SaveStorageChange saves a single storage change
func (r *PostgresRepository) SaveStorageChange(ctx context.Context, change *models.StorageChange) error {
	keyJSON, err := json.Marshal(change.StorageKey)
	if err != nil {
		return fmt.Errorf("failed to marshal storage key: %w", err)
	}
	valueJSON, err2 := json.Marshal(change.StorageValue)
	if err2 != nil {
		return fmt.Errorf("failed to marshal storage value: %w", err2)
	}
	prevJSON, err3 := json.Marshal(change.PreviousValue)
	if err3 != nil {
		return fmt.Errorf("failed to marshal previous value: %w", err3)
	}

	query := `
		INSERT INTO storage_changes (
			contract_id, change_type, storage_key, storage_value, previous_value,
			raw_key, raw_value, raw_previous_value, durability,
			tx_hash, ledger_seq, operation_index, timestamp
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err = r.pool.Exec(ctx, query,
		change.ContractID,
		change.ChangeType,
		keyJSON,
		valueJSON,
		prevJSON,
		change.RawKey,
		change.RawValue,
		change.RawPreviousValue,
		change.Durability,
		change.TxHash,
		change.LedgerSeq,
		change.OperationIndex,
		change.Timestamp,
	)

	if err != nil {
		return fmt.Errorf("failed to save storage change: %w", err)
	}

	return nil
}

// SaveStorageChanges saves multiple storage changes in a transaction
func (r *PostgresRepository) SaveStorageChanges(ctx context.Context, changes []*models.StorageChange) error {
	if len(changes) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO storage_changes (
			contract_id, change_type, storage_key, storage_value, previous_value,
			raw_key, raw_value, raw_previous_value, durability,
			tx_hash, ledger_seq, operation_index, timestamp
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	for _, change := range changes {
		keyJSON, err := json.Marshal(change.StorageKey)
		if err != nil {
			return fmt.Errorf("failed to marshal storage key: %w", err)
		}
		valueJSON, err := json.Marshal(change.StorageValue)
		if err != nil {
			return fmt.Errorf("failed to marshal storage value: %w", err)
		}
		prevJSON, err := json.Marshal(change.PreviousValue)
		if err != nil {
			return fmt.Errorf("failed to marshal previous value: %w", err)
		}

		_, err = tx.Exec(ctx, query,
			change.ContractID,
			change.ChangeType,
			keyJSON,
			valueJSON,
			prevJSON,
			change.RawKey,
			change.RawValue,
			change.RawPreviousValue,
			change.Durability,
			change.TxHash,
			change.LedgerSeq,
			change.OperationIndex,
			change.Timestamp,
		)

		if err != nil {
			return fmt.Errorf("failed to save storage change: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ListStorageChanges lists storage changes for a specific contract with pagination
func (r *PostgresRepository) ListStorageChanges(ctx context.Context, contractID string, limit, offset int) ([]*models.StorageChange, error) {
	query := `
		SELECT
			id, contract_id, change_type, storage_key, storage_value, previous_value,
			raw_key, raw_value, raw_previous_value, durability,
			tx_hash, ledger_seq, operation_index, timestamp, created_at
		FROM storage_changes
		WHERE contract_id = $1
		ORDER BY ledger_seq DESC, id DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, contractID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list storage changes: %w", err)
	}
	defer rows.Close()

	var changes []*models.StorageChange

	for rows.Next() {
		var change models.StorageChange
		var keyJSON, valueJSON, prevJSON []byte

		err := rows.Scan(
			&change.ID,
			&change.ContractID,
			&change.ChangeType,
			&keyJSON,
			&valueJSON,
			&prevJSON,
			&change.RawKey,
			&change.RawValue,
			&change.RawPreviousValue,
			&change.Durability,
			&change.TxHash,
			&change.LedgerSeq,
			&change.OperationIndex,
			&change.Timestamp,
			&change.CreatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan storage change: %w", err)
		}

		// Unmarshal JSON fields
		if len(keyJSON) > 0 {
			if err := json.Unmarshal(keyJSON, &change.StorageKey); err != nil {
				return nil, fmt.Errorf("failed to unmarshal storage key: %w", err)
			}
		}

		if len(valueJSON) > 0 {
			if err := json.Unmarshal(valueJSON, &change.StorageValue); err != nil {
				return nil, fmt.Errorf("failed to unmarshal storage value: %w", err)
			}
		}

		if len(prevJSON) > 0 {
			if err := json.Unmarshal(prevJSON, &change.PreviousValue); err != nil {
				return nil, fmt.Errorf("failed to unmarshal previous value: %w", err)
			}
		}

		changes = append(changes, &change)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating storage changes: %w", err)
	}

	return changes, nil
}

// Ping checks if the database connection is alive
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

// Close closes the database connection pool
func (r *PostgresRepository) Close() error {
	r.pool.Close()
	return nil
}
