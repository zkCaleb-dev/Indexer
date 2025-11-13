# Contract Activity Tracking - Implementation Plan

## Current State

The indexer currently:
- ✅ Detects new contract deployments from factory contracts
- ✅ Extracts complete deployment data including initialization parameters
- ✅ Processes events and storage changes
- ✅ Stable and working correctly

## Goal

Add the ability to track activity (function calls, events, storage changes) on deployed contracts without breaking existing deployment detection.

## Implementation Strategy

This plan follows a **safe, incremental approach** where each phase can be tested independently and rolled back if needed.

---

## Phase 1: Add Infrastructure (No Behavioral Changes)

### Objective
Add the necessary data structures without modifying any existing logic.

### Changes
- Add `trackedContracts map[string]bool` field to `Processor` struct
- Add `mu sync.RWMutex` for thread-safe concurrent access
- Initialize the map in `NewProcessor()`

### Files Modified
- `internal/ledger/processor.go`

### Testing
- Build should succeed
- Existing functionality unchanged
- No new behavior

### Rollback
Simply remove the added fields.

---

## Phase 2: Populate HashSet Silently

### Objective
Start building the list of tracked contracts without using it for detection yet.

### Changes
- When a deployment is successfully detected, add the new contract ID to `trackedContracts`
- Add logging: `"✅ Contract added to tracking: {contractID}"`
- Log HashSet size periodically: `"Currently tracking {count} contracts"`

### Files Modified
- `internal/ledger/processor.go` (in deployment detection logic)

### Testing
- Existing deployment detection works
- New contracts appear in logs as "added to tracking"
- HashSet grows as deployments are detected

### Rollback
Comment out the line that adds contracts to the HashSet.

---

## Phase 3: Add Parallel Detection (Observation Mode)

### Objective
Implement activity detection logic but only log what WOULD be tracked, without actually processing it.

### Changes
- Add new method: `checkTrackedContractActivity(tx, ledgerSeq) bool`
  - Extracts contract IDs from transaction footprint
  - Checks if any are in `trackedContracts` HashSet
  - Returns true if found
- In `processTransaction()`, call this method
- Log: `"📊 Would track activity for contract: {contractID} in tx: {txHash}"`
- **DO NOT** extract or process activity yet

### Files Modified
- `internal/ledger/processor.go`

### Testing
- Deployment detection still works
- Logs show which transactions would be tracked
- Validate that tracked contracts appear in logs when invoked
- No actual activity processing happens

### Rollback
Comment out the call to `checkTrackedContractActivity()`.

---

## Phase 4: Add Configuration Flag

### Objective
Implement actual activity tracking behind a feature flag for safe activation.

### Changes
- Add to `.env` and `internal/config/config.go`:
  ```env
  TRACK_CONTRACT_ACTIVITY=false  # Default: disabled
  ```
- Add `TrackContractActivity bool` field to Config struct
- Modify `processTransaction()`:
  - If `TRACK_CONTRACT_ACTIVITY=true` AND contract is tracked:
    - Call `extractor.ExtractContractActivity()`
    - Log activity details
    - Store/process activity data
  - If `TRACK_CONTRACT_ACTIVITY=false`:
    - Keep observation mode logging only

### Files Modified
- `.env.example`
- `internal/config/config.go`
- `internal/ledger/processor.go`

### Testing
- Test with `TRACK_CONTRACT_ACTIVITY=false`:
  - Should behave exactly like Phase 3 (observation only)
  - Deployments work normally
- Test with `TRACK_CONTRACT_ACTIVITY=true`:
  - Deployments still work
  - Activity on tracked contracts is extracted
  - Activity data is logged/stored

### Rollback
Set `TRACK_CONTRACT_ACTIVITY=false` in `.env`.

---

## Phase 5: Gradual Activation

### Objective
Safely enable activity tracking in production.

### Steps
1. Deploy with `TRACK_CONTRACT_ACTIVITY=false`
2. Monitor logs to confirm deployment detection works
3. Verify tracked contracts list is being populated
4. Set `TRACK_CONTRACT_ACTIVITY=true`
5. Monitor for:
   - Deployment detection still working
   - Activity being captured correctly
   - No performance degradation
   - No errors or crashes
6. If issues arise, immediately set flag back to `false`

### Success Criteria
- ✅ Deployments continue to be detected
- ✅ Activity on deployed contracts is captured
- ✅ Performance is acceptable
- ✅ No errors or data loss

---

## Safety Measures

### Throughout All Phases
1. **Git commits after each phase** with clear descriptions
2. **Test locally** before deploying each phase
3. **Monitor logs** continuously after each deployment
4. **Keep rollback plan** ready for each phase

### Critical Invariants to Maintain
- Deployment detection MUST continue working
- No data loss on existing functionality
- Performance should not degrade significantly
- Application should never crash due to new code

---

## File Structure

```
internal/
├── ledger/
│   ├── processor.go         # Main changes here
│   └── extractor.go          # Already has ExtractContractActivity()
├── config/
│   └── config.go             # Add TRACK_CONTRACT_ACTIVITY flag
└── models/
    └── activity.go           # Already defined ContractActivity model
```

---

## Persistence Considerations (Future Enhancement)

After Phase 5 is stable, consider adding:
- Save `trackedContracts` to JSON file on new deployment
- Load from file on startup
- This prevents losing tracking state on restart

**Format:**
```json
{
  "tracked_contracts": [
    "CONTRACT_ID_1",
    "CONTRACT_ID_2",
    ...
  ],
  "last_updated": "2025-11-13T10:00:00Z"
}
```

---

## Expected Timeline

- **Phase 1**: 15 minutes (pure structure)
- **Phase 2**: 30 minutes (add to HashSet + logging)
- **Phase 3**: 1 hour (detection logic + testing)
- **Phase 4**: 1 hour (config flag + conditional processing)
- **Phase 5**: Variable (monitoring + gradual rollout)

**Total estimated time**: 3-4 hours of development + monitoring time

---

## Notes

- Each phase is independently testable
- Can pause between phases for extended testing
- Feature flag allows instant rollback without code changes
- Existing deployment detection is never modified, only extended
