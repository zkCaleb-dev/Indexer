package extraction

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"indexer/internal/models"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/xdr"
)

// DataExtractor extracts and parses data from ledger transactions into domain models
type DataExtractor struct {
	networkPassphrase string
}

// NewDataExtractor creates a new DataExtractor instance
func NewDataExtractor(networkPassphrase string) *DataExtractor {
	return &DataExtractor{
		networkPassphrase: networkPassphrase,
	}
}

// ExtractDeployedContract extracts complete deployment information from a factory transaction
func (e *DataExtractor) ExtractDeployedContract(
	tx ingest.LedgerTransaction,
	factoryContractID string,
	ledgerSeq uint32,
) (*models.DeployedContract, error) {

	// Extract the new contract ID and initialization params from return value
	newContractID, initParams, err := e.extractDeploymentDataFromReturnValue(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract deployment data: %w", err)
	}

	// Get deployer account
	deployer, _ := tx.Account()

	// Extract initialization events
	events, err := e.ExtractEvents(tx, ledgerSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to extract events: %w", err)
	}

	// Extract initial storage
	storage, err := e.ExtractStorageChanges(tx, ledgerSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to extract storage: %w", err)
	}

	// Get resource usage
	feeCharged, _ := tx.FeeCharged()
	cpuInstructions := uint64(0)
	memoryBytes := uint32(0)

	sorobanData, sorobanOk := tx.GetSorobanData()
	if sorobanOk {
		cpuInstructions = uint64(sorobanData.Resources.Instructions)
		// Approximate memory usage from footprint
		memoryBytes = uint32(len(sorobanData.Resources.Footprint.ReadOnly) + len(sorobanData.Resources.Footprint.ReadWrite))
	}

	// Get memo
	memo := tx.Memo()
	memoType := tx.MemoType()

	contract := &models.DeployedContract{
		ContractID:        newContractID,
		FactoryContractID: factoryContractID,
		DeployedAtLedger:  ledgerSeq,
		DeployedAtTime:    time.Now(), // TODO: Get actual ledger close time
		TxHash:            tx.Hash.HexString(),
		Deployer:          deployer,
		FeeCharged:        feeCharged,
		CPUInstructions:   cpuInstructions,
		MemoryBytes:       memoryBytes,
		InitParams:        initParams,
		InitEvents:        events,
		InitStorage:       storage,
		Memo:              string(memo),
		MemoType:          memoType,
	}

	return contract, nil
}

// extractDeploymentDataFromReturnValue extracts contract ID and initialization params from return value
func (e *DataExtractor) extractDeploymentDataFromReturnValue(tx ingest.LedgerTransaction) (string, map[string]interface{}, error) {
	metaV4, ok := tx.UnsafeMeta.GetV4()
	if !ok {
		return "", nil, fmt.Errorf("transaction meta is not V4")
	}

	if metaV4.SorobanMeta == nil {
		return "", nil, fmt.Errorf("soroban meta is nil")
	}

	returnValue := metaV4.SorobanMeta.ReturnValue

	// In V4, ReturnValue is a pointer
	if returnValue == nil {
		return "", nil, fmt.Errorf("return value is nil")
	}

	// Print the full return value structure for debugging
	e.printScVal("ReturnValue", *returnValue)

	// Check if it's a Vec (complex return) or direct Address (simple return)
	if returnValue.Type == xdr.ScValTypeScvVec {
		// Factory returns a Vec with [contractID, engagementData]
		vec := *returnValue.MustVec()

		if len(vec) == 0 {
			return "", nil, fmt.Errorf("return value is empty vec")
		}

		// Get first element (contract ID)
		firstElement := vec[0]

		slog.Info("Extracting data from Vec",
			"vec_length", len(vec),
			"vec[0]_type", firstElement.Type.String(),
		)

		// Parse the first element as contract ID
		contractID, err := e.parseScValAsContractID(firstElement)
		if err != nil {
			return "", nil, fmt.Errorf("failed to parse vec[0] as contract ID: %w", err)
		}

		// Extract initialization params from second element if available
		var initParams map[string]interface{}
		if len(vec) > 1 {
			secondElement := vec[1]
			slog.Info("Parsing initialization params from Vec[1]",
				"type", secondElement.Type.String(),
			)

			// Parse second element as initialization data
			if parsedData := e.scValToInterface(secondElement); parsedData != nil {
				if dataMap, ok := parsedData.(map[string]interface{}); ok {
					initParams = dataMap
					slog.Info("Successfully parsed init params",
						"fields_count", len(initParams),
					)
				}
			}
		}

		return contractID, initParams, nil
	}

	// Simple case: direct address return (no initialization params)
	contractID, err := e.parseScValAsContractID(*returnValue)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse return value as contract ID: %w", err)
	}

	return contractID, nil, nil
}

// ExtractEvents extracts all contract events from a transaction
func (e *DataExtractor) ExtractEvents(tx ingest.LedgerTransaction, ledgerSeq uint32) ([]models.ContractEvent, error) {
	events, err := tx.GetContractEvents()
	if err != nil {
		return nil, err
	}

	var result []models.ContractEvent
	for i, event := range events {
		parsedEvent, err := e.parseContractEvent(event, tx.Hash.HexString(), ledgerSeq, i)
		if err != nil {
			// Log error but continue with other events
			continue
		}
		result = append(result, parsedEvent)
	}

	return result, nil
}

// parseContractEvent converts an XDR contract event to our model
func (e *DataExtractor) parseContractEvent(
	event xdr.ContractEvent,
	txHash string,
	ledgerSeq uint32,
	eventIndex int,
) (models.ContractEvent, error) {

	// Extract contract ID
	contractID := "unknown"
	if event.ContractId != nil {
		// ContractId is a hash that needs to be encoded
		contractIDBytes, err := event.ContractId.MarshalBinary()
		if err == nil && len(contractIDBytes) > 0 {
			contractID = hex.EncodeToString(contractIDBytes)
		}
	}

	// Parse topics
	topics := make([]string, len(event.Body.V0.Topics))
	rawTopics := make([][]byte, len(event.Body.V0.Topics))
	for i, topic := range event.Body.V0.Topics {
		topics[i] = e.scValToString(topic)
		rawTopics[i], _ = topic.MarshalBinary()
	}

	// Parse data
	data := make(map[string]interface{})
	rawData, _ := event.Body.V0.Data.MarshalBinary()

	// Try to parse data as a structured value
	parsedData := e.scValToInterface(event.Body.V0.Data)
	if parsedData != nil {
		data["parsed"] = parsedData
	}
	data["raw"] = hex.EncodeToString(rawData)

	// Extract event type from first topic (common pattern)
	eventType := "unknown"
	if len(topics) > 0 {
		eventType = topics[0]
	}

	return models.ContractEvent{
		ContractID:               contractID,
		EventType:                eventType,
		EventIndex:               eventIndex,
		Topics:                   topics,
		Data:                     data,
		RawTopics:                rawTopics,
		RawData:                  rawData,
		TxHash:                   txHash,
		LedgerSeq:                ledgerSeq,
		Timestamp:                time.Now(), // TODO: Get actual ledger close time
		InSuccessfulContractCall: true,
	}, nil
}

// ExtractStorageChanges extracts all storage changes from a transaction
func (e *DataExtractor) ExtractStorageChanges(tx ingest.LedgerTransaction, ledgerSeq uint32) ([]models.StorageEntry, error) {
	changes, err := tx.GetChanges()
	if err != nil {
		return nil, err
	}

	var result []models.StorageEntry
	for _, change := range changes {
		// Only process contract data changes
		if change.Type != xdr.LedgerEntryTypeContractData {
			continue
		}

		entry, err := e.parseStorageChange(change, tx.Hash.HexString(), ledgerSeq)
		if err != nil {
			// Log error but continue
			continue
		}
		result = append(result, entry)
	}

	return result, nil
}

// parseStorageChange converts a ledger change to a storage entry
func (e *DataExtractor) parseStorageChange(
	change ingest.Change,
	txHash string,
	ledgerSeq uint32,
) (models.StorageEntry, error) {

	var contractID string
	var key xdr.ScVal
	var value xdr.ScVal
	var changeType models.StorageChangeType
	var previousValue interface{}

	// Determine change type and extract data
	if change.Post != nil {
		contractData := change.Post.Data.ContractData
		contractID, _ = contractData.Contract.String()
		key = contractData.Key
		value = contractData.Val

		if change.Pre == nil {
			changeType = models.StorageCreated
		} else {
			changeType = models.StorageUpdated
			previousValue = e.scValToInterface(change.Pre.Data.ContractData.Val)
		}
	} else if change.Pre != nil {
		// Removed
		contractData := change.Pre.Data.ContractData
		contractID, _ = contractData.Contract.String()
		key = contractData.Key
		changeType = models.StorageRemoved
		previousValue = e.scValToInterface(contractData.Val)
	}

	keyBytes, _ := key.MarshalBinary()
	valueBytes, _ := value.MarshalBinary()

	return models.StorageEntry{
		ContractID:    contractID,
		Key:           hex.EncodeToString(keyBytes),
		KeyType:       e.getScValType(key),
		Value:         e.scValToInterface(value),
		ValueType:     e.getScValType(value),
		RawKey:        keyBytes,
		RawValue:      valueBytes,
		ChangeType:    string(changeType),
		LedgerSeq:     ledgerSeq,
		TxHash:        txHash,
		PreviousValue: previousValue,
	}, nil
}

// Helper functions to parse ScVal

// parseScValAsContractID attempts to parse an ScVal as a contract ID
func (e *DataExtractor) parseScValAsContractID(val xdr.ScVal) (string, error) {
	// Check if it's an address
	if val.Type == xdr.ScValTypeScvAddress {
		addr := val.MustAddress()
		return addr.String()
	}

	return "", fmt.Errorf("not an address type: %v", val.Type)
}

// scValToString converts an ScVal to a string representation
func (e *DataExtractor) scValToString(val xdr.ScVal) string {
	switch val.Type {
	case xdr.ScValTypeScvBool:
		if val.MustB() {
			return "true"
		}
		return "false"
	case xdr.ScValTypeScvVoid:
		return "void"
	case xdr.ScValTypeScvU32:
		return fmt.Sprintf("%d", val.MustU32())
	case xdr.ScValTypeScvI32:
		return fmt.Sprintf("%d", val.MustI32())
	case xdr.ScValTypeScvU64:
		return fmt.Sprintf("%d", val.MustU64())
	case xdr.ScValTypeScvI64:
		return fmt.Sprintf("%d", val.MustI64())
	case xdr.ScValTypeScvSymbol:
		return string(val.MustSym())
	case xdr.ScValTypeScvString:
		return string(val.MustStr())
	case xdr.ScValTypeScvAddress:
		addr := val.MustAddress()
		str, _ := addr.String()
		return str
	case xdr.ScValTypeScvBytes:
		return hex.EncodeToString(val.MustBytes())
	default:
		return fmt.Sprintf("<%s>", val.Type.String())
	}
}

// scValToInterface converts an ScVal to a Go interface{} for JSON serialization
func (e *DataExtractor) scValToInterface(val xdr.ScVal) interface{} {
	switch val.Type {
	case xdr.ScValTypeScvBool:
		return val.MustB()
	case xdr.ScValTypeScvVoid:
		return nil
	case xdr.ScValTypeScvU32:
		return val.MustU32()
	case xdr.ScValTypeScvI32:
		return val.MustI32()
	case xdr.ScValTypeScvU64:
		return val.MustU64()
	case xdr.ScValTypeScvI64:
		return val.MustI64()
	case xdr.ScValTypeScvU128:
		// U128 is stored as hi and lo uint64s
		u128 := val.MustU128()
		return map[string]interface{}{
			"hi":  uint64(u128.Hi),
			"lo":  uint64(u128.Lo),
			"hex": fmt.Sprintf("%016x%016x", u128.Hi, u128.Lo),
		}
	case xdr.ScValTypeScvI128:
		// I128 is stored as hi and lo int64s
		i128 := val.MustI128()
		return map[string]interface{}{
			"hi":  int64(i128.Hi),
			"lo":  uint64(i128.Lo),
			"hex": fmt.Sprintf("%016x%016x", i128.Hi, i128.Lo),
		}
	case xdr.ScValTypeScvU256:
		// U256 is stored as 4 uint64s
		u256 := val.MustU256()
		return map[string]interface{}{
			"parts": []uint64{uint64(u256.HiHi), uint64(u256.HiLo), uint64(u256.LoHi), uint64(u256.LoLo)},
			"hex":   fmt.Sprintf("%016x%016x%016x%016x", u256.HiHi, u256.HiLo, u256.LoHi, u256.LoLo),
		}
	case xdr.ScValTypeScvI256:
		// I256 is stored as 4 uint64s (with hi being signed)
		i256 := val.MustI256()
		return map[string]interface{}{
			"parts": []int64{int64(i256.HiHi), int64(i256.HiLo), int64(i256.LoHi), int64(i256.LoLo)},
			"hex":   fmt.Sprintf("%016x%016x%016x%016x", i256.HiHi, i256.HiLo, i256.LoHi, i256.LoLo),
		}
	case xdr.ScValTypeScvSymbol:
		return string(val.MustSym())
	case xdr.ScValTypeScvString:
		return string(val.MustStr())
	case xdr.ScValTypeScvAddress:
		addr := val.MustAddress()
		str, _ := addr.String()
		return str
	case xdr.ScValTypeScvBytes:
		return hex.EncodeToString(val.MustBytes())
	case xdr.ScValTypeScvVec:
		// Parse vec recursively
		vec := *val.MustVec()
		result := make([]interface{}, len(vec))
		for i, element := range vec {
			result[i] = e.scValToInterface(element)
		}
		return result
	case xdr.ScValTypeScvMap:
		// Parse map recursively
		scMap := *val.MustMap()
		result := make(map[string]interface{})
		for _, entry := range scMap {
			// Keys are typically symbols or strings
			keyStr := e.scValToString(entry.Key)
			result[keyStr] = e.scValToInterface(entry.Val)
		}
		return result
	default:
		return val.Type.String()
	}
}

// getScValType returns the type name of an ScVal
func (e *DataExtractor) getScValType(val xdr.ScVal) string {
	return val.Type.String()
}

// printScVal prints the complete structure of an ScVal for debugging
func (e *DataExtractor) printScVal(label string, val xdr.ScVal) {
	slog.Info("=== ScVal Debug ===",
		"label", label,
		"type", val.Type.String(),
	)

	// Try to marshal to XDR base64 for debugging
	if bytes, err := val.MarshalBinary(); err == nil {
		slog.Info("ScVal XDR bytes",
			"label", label,
			"hex", hex.EncodeToString(bytes),
		)
	}

	switch val.Type {
	case xdr.ScValTypeScvVec:
		slog.Info("Return value is a Vec (printing as generic)",
			"label", label,
		)
		// Print generic representation
		slog.Info("Vec content", "generic", e.scValToInterface(val))

	case xdr.ScValTypeScvMap:
		slog.Info("Return value is a Map (printing as generic)",
			"label", label,
			"generic", e.scValToInterface(val),
		)
	case xdr.ScValTypeScvAddress:
		if addr, err := val.MustAddress().String(); err == nil {
			slog.Info("Return value is an Address",
				"label", label,
				"address", addr,
			)
		}
	default:
		slog.Info("Return value",
			"label", label,
			"type", val.Type.String(),
			"value", e.scValToInterface(val),
		)
	}
}

// ExtractContractActivity extracts activity information from any contract interaction
func (e *DataExtractor) ExtractContractActivity(
	tx ingest.LedgerTransaction,
	contractID string,
	ledgerSeq uint32,
) (*models.ContractActivity, error) {

	events, err := e.ExtractEvents(tx, ledgerSeq)
	if err != nil {
		return nil, err
	}

	storage, err := e.ExtractStorageChanges(tx, ledgerSeq)
	if err != nil {
		return nil, err
	}

	// Extract return value
	var returnValue interface{}
	if metaV4, ok := tx.UnsafeMeta.GetV4(); ok {
		if metaV4.SorobanMeta != nil && metaV4.SorobanMeta.ReturnValue != nil {
			returnValue = e.scValToInterface(*metaV4.SorobanMeta.ReturnValue)
		}
	}

	invoker, _ := tx.Account()
	feeCharged, _ := tx.FeeCharged()

	activity := &models.ContractActivity{
		ActivityID:      fmt.Sprintf("%s:%d", tx.Hash.HexString(), 0),
		ContractID:      contractID,
		ActivityType:    string(models.ActivityInvocation),
		TxHash:          tx.Hash.HexString(),
		LedgerSeq:       ledgerSeq,
		Timestamp:       time.Now(), // TODO: Get actual ledger close time
		Invoker:         invoker,
		Success:         tx.Successful(),
		ReturnValue:     returnValue,
		Events:          events,
		StorageChanges:  storage,
		FeeCharged:      feeCharged,
	}

	// Get resource usage
	sorobanData, sorobanOk := tx.GetSorobanData()
	if sorobanOk {
		activity.CPUInstructions = uint64(sorobanData.Resources.Instructions)
		activity.MemoryBytes = uint32(len(sorobanData.Resources.Footprint.ReadOnly) + len(sorobanData.Resources.Footprint.ReadWrite))
	}

	return activity, nil
}
