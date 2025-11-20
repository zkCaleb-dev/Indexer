# Stellar Indexer - System Design Document

## Executive Summary

A high-performance, resilient Stellar blockchain indexer built with Go that monitors smart contract deployments and activities. Features adaptive parallel processing, comprehensive observability, and automatic checkpoint/resume capabilities.

---

## Architecture Overview

```mermaid
graph TB
    subgraph "External Dependencies"
        STELLAR[Stellar RPC API<br/>soroban-testnet.stellar.org]
        POSTGRES[(PostgreSQL<br/>Database)]
    end

    subgraph "Configuration Layer"
        ENV[.env Configuration]
        CONFIG[Config Loader<br/>internal/config]
        ENV -->|Load Environment| CONFIG
    end

    subgraph "Main Application - cmd/indexer/main.go"
        MAIN[Main Entry Point]
        MAIN -->|1. Load Config| CONFIG
        MAIN -->|2. Initialize| POSTGRES
        MAIN -->|3. Create Components| BACKEND
        MAIN -->|4. Start Streaming| STREAMER
        MAIN -->|5. Start Observability| API
    end

    subgraph "Data Ingestion Layer"
        BACKEND[RPCLedgerBackend<br/>Buffer: 150 ledgers]
        STREAMER[Streamer<br/>Checkpoint every 100 ledgers]
        RETRY[Retry Strategy<br/>Exponential Backoff]

        STELLAR -->|Fetch Ledgers| BACKEND
        BACKEND -->|Stream Ledgers| STREAMER
        STREAMER -->|With Retry Logic| RETRY
    end

    subgraph "Processing Mode Controller"
        DETECTOR{Lag Detector<br/>Check every 50 ledgers}
        STREAMER -->|Monitor Lag| DETECTOR
        DETECTOR -->|Lag > 100| PARALLEL_MODE
        DETECTOR -->|Lag < 10| SEQUENTIAL_MODE
    end

    subgraph "Sequential Processing Mode"
        SEQUENTIAL_MODE[Sequential Processor]
        PROCESSOR[Ledger Processor<br/>internal/ledger]

        SEQUENTIAL_MODE -->|Process Ledger| PROCESSOR
        PROCESSOR -->|Execute Services| ORCHESTRATOR
    end

    subgraph "Parallel Processing Mode"
        PARALLEL_MODE[Parallel Pipeline<br/>75% CPU cores]
        WORKERS[Worker Pool<br/>N Workers]
        ORDERER[Result Orderer<br/>Maintains Sequence]

        PARALLEL_MODE -->|Distribute| WORKERS
        WORKERS -->|Out-of-order Results| ORDERER
        ORDERER -->|Ordered Commits| POSTGRES
    end

    subgraph "Service Orchestration Layer - Chain of Responsibility"
        ORCHESTRATOR[Orchestrator<br/>Coordinates Services]

        FACTORY[1. Factory Service<br/>Detect Deployments]
        ACTIVITY[2. Activity Service<br/>Track Contract Activity]
        EVENTS[3. Event Service<br/>Extract tw_* Events]
        STORAGE[4. Storage Change Service<br/>Extract Storage Deltas]

        ORCHESTRATOR -->|Execute in Order| FACTORY
        FACTORY -->|Notify Deployment| ACTIVITY
        ACTIVITY -->|Track Contract| EVENTS
        ACTIVITY -->|Track Contract| STORAGE
    end

    subgraph "Data Access Layer"
        REPO[Repository Interface<br/>internal/storage]

        FACTORY -->|Save Contract| REPO
        ACTIVITY -->|Save Activity| REPO
        EVENTS -->|Save Events| REPO
        STORAGE -->|Save Changes| REPO
        REPO -->|Persist| POSTGRES
        STREAMER -->|Save Checkpoint| REPO
    end

    subgraph "Observability & API Layer"
        API[REST API Server<br/>Port 2112]
        METRICS[Prometheus Metrics<br/>/metrics]
        HEALTH[Health Check<br/>/health]
        ENDPOINTS[REST Endpoints<br/>Contract Queries]

        API -->|Expose| METRICS
        API -->|Expose| HEALTH
        API -->|Expose| ENDPOINTS
        ENDPOINTS -->|Query| REPO
    end

    style MAIN fill:#ff6b6b
    style STREAMER fill:#4ecdc4
    style DETECTOR fill:#ffe66d
    style PARALLEL_MODE fill:#95e1d3
    style SEQUENTIAL_MODE fill:#95e1d3
    style ORCHESTRATOR fill:#f38181
    style POSTGRES fill:#aa96da
    style API fill:#fcbad3
```

---

## Detailed Component Architecture

### 1. Data Ingestion Flow

```mermaid
sequenceDiagram
    participant RPC as Stellar RPC
    participant Backend as RPCLedgerBackend
    participant Streamer as Streamer
    participant Retry as Retry Strategy
    participant Processor as Processor/Pipeline

    Note over Streamer: Start from checkpoint or config

    Streamer->>Backend: PrepareRange(startLedger, unbounded)
    activate Backend
    Backend->>RPC: Prefetch 150 ledgers into buffer
    RPC-->>Backend: Ledger data (XDR)
    deactivate Backend

    loop Every Ledger
        Streamer->>Backend: GetLedger(sequence)
        Backend-->>Streamer: LedgerCloseMeta (from buffer)

        alt Fetch Failed
            Streamer->>Retry: Execute with backoff
            Retry->>Backend: Retry GetLedger
            Backend->>RPC: Fetch again
        end

        alt Lag > 100 ledgers
            Streamer->>Processor: Switch to Parallel Mode
            Note over Streamer: Auto-enable pipeline
        else Lag < 10 ledgers
            Streamer->>Processor: Switch to Sequential Mode
            Note over Streamer: Auto-disable pipeline
        end

        Streamer->>Processor: Process(ledger)

        alt Every 100 ledgers
            Streamer->>Database: SaveProgress(sequence)
            Note over Streamer: Checkpoint for resume
        end
    end
```

### 2. Service Processing Pipeline

```mermaid
flowchart LR
    subgraph "Input"
        TX[Transaction<br/>LedgerTransaction]
    end

    subgraph "Orchestrator Loop"
        ORCH[Orchestrator<br/>Process Each Service]
    end

    subgraph "Service Chain - Chain of Responsibility Pattern"
        direction TB

        S1[Factory Service]
        S1_CHECK{Is Factory<br/>Deployment?}
        S1_ACTION[Save Contract<br/>Add to Tracking]

        S2[Activity Service]
        S2_CHECK{Is Tracked<br/>Contract?}
        S2_ACTION[Record Activity<br/>Update LastActive]

        S3[Event Service]
        S3_CHECK{Has tw_*<br/>Events?}
        S3_ACTION[Extract & Save<br/>Filtered Events]

        S4[Storage Change Service]
        S4_CHECK{Has Storage<br/>Changes?}
        S4_ACTION[Extract & Save<br/>Storage Deltas]

        S1 --> S1_CHECK
        S1_CHECK -->|Yes| S1_ACTION
        S1_ACTION -->|Notify| S2
        S1_CHECK -->|No| S2

        S2 --> S2_CHECK
        S2_CHECK -->|Yes| S2_ACTION
        S2_ACTION -->|Propagate| S3
        S2_CHECK -->|No| S3

        S3 --> S3_CHECK
        S3_CHECK -->|Yes| S3_ACTION
        S3_ACTION --> S4
        S3_CHECK -->|No| S4

        S4 --> S4_CHECK
        S4_CHECK -->|Yes| S4_ACTION
    end

    subgraph "Ledger Flush"
        FLUSH{Ledger<br/>Changed?}
        FLUSH_ACTION["FlushLedger()<br/>Batch Commit"]
    end

    TX --> ORCH
    ORCH --> S1
    S4_ACTION --> FLUSH
    S4_CHECK -->|No| FLUSH
    FLUSH -->|Yes| FLUSH_ACTION

    style S1_ACTION fill:#95e1d3
    style S2_ACTION fill:#95e1d3
    style S3_ACTION fill:#95e1d3
    style S4_ACTION fill:#95e1d3
    style FLUSH_ACTION fill:#f38181
```

### 3. Parallel Processing Pipeline

```mermaid
graph TB
    subgraph "Streamer - Main Thread"
        STREAM[Ledger Stream]
        SUBMIT[Submit to Pipeline]
    end

    subgraph "Pipeline Controller"
        ENABLED{Pipeline<br/>Enabled?}
        LAG_CHECK{Check Lag<br/>Every 50 ledgers}
    end

    subgraph "Worker Pool - Concurrent Processing"
        W1[Worker 1<br/>goroutine]
        W2[Worker 2<br/>goroutine]
        W3[Worker N<br/>goroutine]

        W1 -->|Process| P1[Processor Copy]
        W2 -->|Process| P2[Processor Copy]
        W3 -->|Process| P3[Processor Copy]
    end

    subgraph "Result Ordering"
        RESULTS[Results Channel<br/>Out-of-order]
        ORDERER[Orderer<br/>Sequence Enforcer]
        BUFFER[Pending Buffer<br/>Future Results]
        COMMIT[Sequential Commit]
    end

    subgraph "Database"
        DB[(PostgreSQL<br/>ACID Guarantees)]
    end

    STREAM --> LAG_CHECK
    LAG_CHECK -->|Lag > 100| ENABLED
    ENABLED -->|Yes| SUBMIT

    SUBMIT -->|Distribute| W1
    SUBMIT -->|Distribute| W2
    SUBMIT -->|Distribute| W3

    W1 -->|Result Seq: 105| RESULTS
    W2 -->|Result Seq: 103| RESULTS
    W3 -->|Result Seq: 104| RESULTS

    RESULTS --> ORDERER
    ORDERER -->|Check Next Expected| BUFFER
    BUFFER -->|Seq 103 ready| COMMIT
    COMMIT -->|Next: 104| BUFFER
    BUFFER -->|Seq 104 ready| COMMIT
    COMMIT -->|Next: 105| BUFFER
    BUFFER -->|Seq 105 ready| COMMIT

    COMMIT -->|Ordered Writes| DB

    style SUBMIT fill:#ffe66d
    style W1 fill:#95e1d3
    style W2 fill:#95e1d3
    style W3 fill:#95e1d3
    style ORDERER fill:#f38181
    style COMMIT fill:#4ecdc4
```

---

## Component Details

### Core Components

#### 1. **Streamer** (`internal/ledger/streamer.go`)
**Responsibilities:**
- Main event loop for ledger ingestion
- Checkpoint management (save progress every 100 ledgers)
- Adaptive mode switching (sequential ↔ parallel)
- Lag detection and automatic pipeline activation

**Key Actions:**
- `PrepareRange(unbounded)` - Initialize streaming from start ledger
- `GetLedger(sequence)` - Fetch ledger with retry logic
- `ShouldEnableParallel()` - Check lag every 50 ledgers
- `SaveProgress()` - Checkpoint for crash recovery

#### 2. **Processor** (`internal/ledger/processor.go`)
**Responsibilities:**
- Parse ledger XDR data
- Extract Soroban transactions
- Normalize transaction data for services
- Invoke orchestrator for each transaction

**Key Actions:**
- `Process(ledger)` - Main processing entry point
- `extractTransactions()` - Parse XDR to ingest.LedgerTransaction
- `normalizeTransaction()` - Create ProcessedTx with metadata
- `orchestrator.ProcessTx()` - Delegate to services

#### 3. **Orchestrator** (`internal/orchestrator/orchestrator.go`)
**Responsibilities:**
- Coordinate service execution order
- Manage ledger boundaries
- Handle service failures gracefully
- Trigger batch flushes

**Key Actions:**
- `ProcessTx(tx)` - Execute service chain
- `flushLedger()` - Trigger batch commits on ledger change
- Error handling: Continue on service failure, log for observability

#### 4. **Pipeline** (`internal/pipeline/pipeline.go`)
**Responsibilities:**
- Parallel ledger processing during catch-up
- Worker pool management (75% of CPU cores)
- Result ordering and sequential commits
- Auto-enable/disable based on network lag

**Key Actions:**
- `StartParallel()` - Spin up worker pool
- `SubmitLedger()` - Distribute work to workers
- `runOrderer()` - Enforce sequential database writes
- `Stop()` - Graceful shutdown

---

### Service Layer

Services implement the `Service` interface and are executed by the Orchestrator in a **Chain of Responsibility** pattern.

#### Service Chain Order:

```mermaid
graph LR
    F[1. Factory Service] -->|deployment detected| A[2. Activity Service]
    A -->|tracking enabled| E[3. Event Service]
    A -->|tracking enabled| S[4. Storage Change Service]

    style F fill:#ff6b6b
    style A fill:#4ecdc4
    style E fill:#ffe66d
    style S fill:#95e1d3
```

#### 1. **Factory Service** (`internal/services/factory_service.go`)
**Purpose:** Detect new smart contract deployments from factory contracts

**Key Actions:**
- `Process(tx)` - Check if transaction invokes factory contract
- `isFactoryDeployment()` - Parse contract call for deployment methods
- `extractDeployedContract()` - Get new contract ID from footprint
- `SaveContract()` - Persist to database
- `NotifyActivityService()` - Enable tracking for new contract

**Factory Types Supported:**
- `single-release` - One contract per deployment
- `multi-release` - Multiple contract deployments

#### 2. **Activity Service** (`internal/services/activity_service.go`)
**Purpose:** Track contract activity and maintain "last active" timestamps

**Key Actions:**
- `Process(tx)` - Check if transaction touches tracked contracts
- `AddTrackedContract()` - Add contract to in-memory tracking set
- `isTrackedContract()` - Fast lookup in contract ID set
- `RecordActivity()` - Update last_active_at timestamp
- `PropagateTracking()` - Notify Event + Storage services

**Tracking Strategy:**
- In-memory `map[string]bool` for fast lookups
- Load existing contracts on startup from database
- Auto-add new deployments from Factory Service

#### 3. **Event Service** (`internal/services/event_service.go`)
**Purpose:** Extract and filter Soroban events (only `tw_*` prefixed events)

**Key Actions:**
- `Process(tx)` - Extract events from transaction meta
- `filterEvents()` - Keep only events with `tw_` topic prefix
- `SaveEvents()` - Batch insert to database
- `FlushLedger()` - Commit batch on ledger change

**Event Filtering:**
```go
// Only save events with topics starting with "tw_"
// Example: tw_transfer, tw_mint, tw_burn
if strings.HasPrefix(topic, "tw_") {
    saveEvent(event)
}
```

#### 4. **Storage Change Service** (`internal/services/storage_change_service.go`)
**Purpose:** Track contract storage mutations for state analysis

**Key Actions:**
- `Process(tx)` - Extract storage changes from footprint
- `categorizeChanges()` - Identify created/updated/deleted keys
- `SaveStorageChanges()` - Batch insert deltas
- `FlushLedger()` - Commit batch on ledger change

**Storage Change Types:**
- `created` - New storage key
- `updated` - Existing key modified
- `deleted` - Key removed

---

### Data Access Layer

#### Repository Pattern (`internal/storage/repository.go` + `postgres.go`)

**Interface Abstraction:**
```go
type Repository interface {
    SaveContract(ctx, contract) error
    SaveActivity(ctx, activity) error
    SaveEvents(ctx, events) error
    SaveStorageChanges(ctx, changes) error

    GetTrackedContractIDs(ctx) ([]string, error)
    GetProgress(ctx) (ledger, exists, error)
    SaveProgress(ctx, ledger) error
}
```

**Implementation:** PostgreSQL with connection pooling

**Key Features:**
- Batch inserts for performance
- ACID transactions
- Migration-based schema evolution
- Checkpoint/resume support

---

## Database Schema

```mermaid
erDiagram
    CONTRACTS ||--o{ CONTRACT_ACTIVITY : tracks
    CONTRACTS ||--o{ CONTRACT_EVENTS : emits
    CONTRACTS ||--o{ STORAGE_CHANGES : modifies
    INDEXER_PROGRESS ||--|| CONTRACTS : checkpoints

    CONTRACTS {
        string contract_id PK
        string factory_id FK
        string contract_type
        uint32 deployed_ledger
        timestamp deployed_at
        timestamp created_at
    }

    CONTRACT_ACTIVITY {
        bigserial id PK
        string contract_id FK
        uint32 ledger_sequence
        timestamp ledger_close_time
        string transaction_hash
        timestamp last_active_at
        timestamp created_at
    }

    CONTRACT_EVENTS {
        bigserial id PK
        string contract_id FK
        uint32 ledger_sequence
        string transaction_hash
        string event_type
        jsonb topics
        jsonb data
        timestamp ledger_close_time
        timestamp created_at
    }

    STORAGE_CHANGES {
        bigserial id PK
        string contract_id FK
        uint32 ledger_sequence
        string transaction_hash
        string storage_key
        string change_type
        string durability
        string value_before
        string value_after
        timestamp ledger_close_time
        timestamp created_at
    }

    INDEXER_PROGRESS {
        int id PK
        uint32 last_processed_ledger
        timestamp updated_at
    }
```

**Migration Files:**
- `001_initial_schema.sql` - Base tables
- `002_add_storage_changes.sql` - Storage tracking
- `003-007` - Schema refinements (contract ID format, durability, indexes)

---

## Configuration & Operational Concerns

### Configuration (`internal/config/config.go`)

**Environment Variables:**
```bash
# Stellar Network
RPC_SERVER_URL=https://soroban-testnet.stellar.org
NETWORK_PASSPHRASE=Test SDF Network ; September 2015
START_LEDGER=0  # 0 = auto-detect from latest

# Performance Tuning
BUFFER_SIZE=150                    # Ledger prefetch buffer
HTTP_TIMEOUT_SEC=60
HTTP_MAX_IDLE_CONNS=100
HTTP_MAX_CONNS_PER_HOST=100

# Checkpointing
CHECKPOINT_INTERVAL=100            # Save progress every N ledgers

# Parallel Processing
ENABLE_PARALLEL_PROCESSING=true
PIPELINE_WORKER_COUNT=0            # 0 = auto (75% CPU cores)
AUTO_ENABLE_LAG_THRESHOLD=100      # Enable if lag > 100
AUTO_DISABLE_LAG_THRESHOLD=10      # Disable if lag < 10

# Factory Contracts
FACTORY_CONTRACT_SINGLE_RELEASE_ID=CDQPREX...
FACTORY_CONTRACT_MULTI_RELEASE_ID=CCAJPWPKSR...

# Database
DATABASE_URL=postgresql://user:pass@localhost:5433/stellar_indexer

# API Server
API_SERVER_PORT=2112
LOG_LEVEL=info
```

---

### Retry Strategy (`internal/ledger/retry/`)

**Exponential Backoff Configuration:**
```plain text
InitialDelay:  1s
MaxDelay:      30s
MaxRetries:    10
Multiplier:    2.0
```

**Retry Flow:**
```mermaid
graph LR
    A[Attempt] -->|Failure| B{Retry < Max?}
    B -->|Yes| C["Wait: delay × 2^attempt"]
    C --> A
    B -->|No| D[Fatal Error]
    A -->|Success| E[Continue]

    style D fill:#ff6b6b
    style E fill:#95e1d3
```

---

### Observability

#### Prometheus Metrics (`internal/metrics/metrics.go`)

**Available Metrics:**
```
# System Metrics
indexer_buffer_size                     # Ledger buffer configuration
indexer_tracked_contracts               # Number of contracts being monitored

# Pipeline Metrics
indexer_pipeline_mode                   # 0=sequential, 1=parallel
indexer_pipeline_worker_count           # Active worker count
indexer_pipeline_lag                    # Network lag (ledgers behind)
indexer_pipeline_queue_depth            # Pending ledgers in queue

# Processing Metrics
indexer_ledgers_processed_total         # Total ledgers indexed
indexer_transactions_processed_total    # Total transactions
indexer_errors_total                    # Processing errors

# Performance Metrics
indexer_ledger_fetch_duration_seconds   # RPC fetch latency
indexer_ledger_process_duration_seconds # Processing latency
```

#### REST API Endpoints (Port 2112)

```
GET /metrics                 # Prometheus metrics
GET /health                  # Health check
GET /api/v1/contracts        # List tracked contracts
GET /api/v1/contracts/:id    # Get contract details
GET /api/v1/events           # Query events (filterable)
GET /api/v1/storage-changes  # Query storage deltas
```

---

## Operational Flows

### 1. Startup Sequence

```mermaid
sequenceDiagram
    participant Main as main.go
    participant Config as Config
    participant DB as Database
    participant Repo as Repository
    participant Streamer as Streamer
    participant API as API Server

    Main->>Config: Load()
    Config-->>Main: Configuration

    Main->>Config: Validate()

    Main->>DB: Connect(DATABASE_URL)
    DB-->>Main: Connection Pool

    Main->>Repo: NewPostgresRepository()

    Main->>Repo: GetProgress()
    alt Checkpoint exists
        Repo-->>Main: Resume from ledger N+1
    else No checkpoint
        Main->>Config: Use START_LEDGER or auto-detect
    end

    Main->>Repo: GetTrackedContractIDs()
    Repo-->>Main: Existing contracts
    Main->>Main: Load contracts into ActivityService

    Main->>Streamer: NewStreamer(config)
    Main->>API: Start(port 2112)

    Main->>Streamer: Start(startLedger)
    Note over Streamer: Begin streaming loop

    Main->>Main: Wait for SIGTERM/SIGINT
```

### 2. Graceful Shutdown

```mermaid
sequenceDiagram
    participant Signal as OS Signal
    participant Main as main.go
    participant Streamer as Streamer
    participant Pipeline as Pipeline
    participant DB as Database
    participant API as API Server

    Signal->>Main: SIGTERM / SIGINT

    Main->>Streamer: Stop()

    alt Pipeline is running
        Streamer->>Pipeline: Stop()
        Pipeline->>Pipeline: Close ledger channel
        Pipeline->>Pipeline: Wait for workers
        Pipeline->>DB: Flush pending results
    end

    Streamer->>DB: SaveProgress(lastLedger)
    Note over DB: Checkpoint saved for resume

    Main->>API: Shutdown(5s timeout)
    API->>API: Finish pending requests

    Main->>DB: Close()

    Main->>Main: Exit(0)
```

### 3. Error Handling Strategy

```mermaid
graph TD
    ERROR{Error Type}

    ERROR -->|Network Error| RETRY[Retry with<br/>Exponential Backoff]
    ERROR -->|Service Error| LOG[Log Error<br/>Continue Processing]
    ERROR -->|Database Error| FATAL[Fatal Error<br/>Stop Indexer]
    ERROR -->|Context Cancelled| GRACEFUL[Graceful Shutdown]

    RETRY -->|Max Retries| FATAL
    RETRY -->|Success| CONTINUE[Continue]
    LOG --> CONTINUE

    style FATAL fill:#ff6b6b
    style CONTINUE fill:#95e1d3
```

**Error Categories:**

| Error Type | Action | Rationale |
|------------|--------|-----------|
| RPC Timeout | Retry with backoff | Transient network issue |
| Service Processing Failure | Log + Continue | Don't stop indexer for one bad tx |
| Database Connection Loss | Fatal stop | Data integrity at risk |
| Context Cancelled | Graceful shutdown | User-initiated stop |

---

## Performance Characteristics

### Sequential Mode
- **Throughput:** ~1-2 ledgers/second
- **Latency:** ~500ms per ledger
- **CPU Usage:** Single core (main goroutine)
- **Use Case:** Real-time tracking when caught up with network

### Parallel Mode (Auto-enabled when lag > 100)
- **Throughput:** ~5-20 ledgers/second (depends on CPU cores)
- **Latency:** Variable (out-of-order processing)
- **CPU Usage:** 75% of available cores
- **Use Case:** Catch-up after restart or network lag

### Memory Profile
- **Ledger Buffer:** ~150 MB (150 ledgers × ~1MB each)
- **Worker Pool:** ~50 MB per worker (goroutine + buffers)
- **Database Connections:** Pooled (max 100)
- **Total (8 cores):** ~400-600 MB

### Database Performance
- **Batch Inserts:** Services use `FlushLedger()` for batch commits
- **Indexes:** Contract ID, Ledger Sequence, Transaction Hash
- **Connection Pool:** 100 max idle connections
- **Checkpoint Interval:** Every 100 ledgers (balance freshness vs I/O)

---

## Failure Scenarios & Recovery

### 1. Indexer Crash
**Recovery:**
1. Read `indexer_progress` table on restart
2. Resume from `last_processed_ledger + 1`
3. Reload tracked contracts from database
4. Continue streaming

**Data Loss:** None (last checkpoint to crash window only)

### 2. Database Unavailability
**Behavior:**
- Fatal error - stop indexer
- No partial commits (ACID guarantees)

**Recovery:**
1. Fix database connectivity
2. Restart indexer
3. Resume from checkpoint

### 3. RPC Service Degradation
**Behavior:**
- Retry with exponential backoff
- Max retries: 10 (up to 30s delays)
- If all retries fail: fatal error

**Mitigation:**
- Use reliable RPC endpoint
- Consider fallback RPC URLs (future enhancement)

### 4. High Network Lag
**Automatic Response:**
1. Detect lag > 100 ledgers
2. Auto-enable parallel pipeline
3. Catch up faster with worker pool
4. Auto-disable when lag < 10

---

## Design Patterns Used

### 1. **Chain of Responsibility**
- **Location:** Service orchestration
- **Implementation:** Orchestrator → Factory → Activity → Events → Storage
- **Benefit:** Decoupled service logic, easy to add/remove services

### 2. **Repository Pattern**
- **Location:** Data access layer
- **Implementation:** `Repository` interface + `PostgresRepository`
- **Benefit:** Database abstraction, testability, potential multi-DB support

### 3. **Strategy Pattern**
- **Location:** Retry logic
- **Implementation:** `RetryStrategy` interface (Exponential, NoRetry)
- **Benefit:** Pluggable retry behavior

### 4. **Worker Pool Pattern**
- **Location:** Parallel pipeline
- **Implementation:** N workers + work queue + result orderer
- **Benefit:** Parallelism with ordered commits

### 5. **Observer Pattern**
- **Location:** Service notifications
- **Implementation:** Factory → Activity, Activity → Events/Storage
- **Benefit:** Loosely coupled service communication

### 6. **Template Method**
- **Location:** Service interface
- **Implementation:** `Process()` + optional `FlushLedger()`
- **Benefit:** Consistent service lifecycle

---

## Future Enhancements (Potential)

### Scalability
- [ ] **Horizontal Scaling:** Multiple indexer instances with ledger range partitioning
- [ ] **Sharded Database:** Partition by contract ID or ledger range
- [ ] **Read Replicas:** Separate read/write database connections

### Reliability
- [ ] **Multi-RPC Failover:** Automatic switching between RPC endpoints
- [ ] **Dead Letter Queue:** Failed transactions for manual review
- [ ] **Circuit Breaker:** Protect against cascading failures

### Observability
- [ ] **Distributed Tracing:** OpenTelemetry integration
- [ ] **Alerting:** Prometheus Alertmanager rules
- [ ] **Dashboard:** Grafana dashboard for ops team

### Features
- [ ] **GraphQL API:** More flexible querying than REST
- [ ] **Webhook Support:** Real-time notifications for contract events
- [ ] **Contract Indexing:** Full-text search on contract data

---

## Deployment Architecture (Production-Ready)

```mermaid
graph TB
    subgraph "Load Balancer"
        LB[Nginx / HAProxy]
    end

    subgraph "Indexer Cluster"
        I1[Indexer Instance 1<br/>Ledgers 0-1M]
        I2[Indexer Instance 2<br/>Ledgers 1M-2M]
        I3[Indexer Instance N<br/>Ledgers 2M+]
    end

    subgraph "Database Cluster"
        PRIMARY[(PostgreSQL Primary<br/>Writes)]
        REPLICA1[(Replica 1<br/>Reads)]
        REPLICA2[(Replica 2<br/>Reads)]

        PRIMARY -.->|Replication| REPLICA1
        PRIMARY -.->|Replication| REPLICA2
    end

    subgraph "Monitoring Stack"
        PROM[Prometheus]
        GRAFANA[Grafana]
        ALERT[Alertmanager]

        PROM --> GRAFANA
        PROM --> ALERT
    end

    subgraph "External Services"
        RPC[Stellar RPC<br/>Load Balanced]
    end

    RPC -->|Fetch Ledgers| I1
    RPC -->|Fetch Ledgers| I2
    RPC -->|Fetch Ledgers| I3

    I1 -->|Write| PRIMARY
    I2 -->|Write| PRIMARY
    I3 -->|Write| PRIMARY

    LB -->|Read Queries| REPLICA1
    LB -->|Read Queries| REPLICA2

    I1 -->|/metrics| PROM
    I2 -->|/metrics| PROM
    I3 -->|/metrics| PROM

    style PRIMARY fill:#aa96da
    style I1 fill:#4ecdc4
    style I2 fill:#4ecdc4
    style I3 fill:#4ecdc4
    style PROM fill:#fcbad3
```

---

## Technology Stack

| Component | Technology | Version | Purpose |
|-----------|-----------|---------|---------|
| Language | Go | 1.25+ | High-performance, concurrent execution |
| Database | PostgreSQL | 14+ | ACID compliance, JSON support |
| RPC Client | Stellar Go SDK | Latest | Ledger ingestion |
| API Framework | Native `net/http` | Go stdlib | REST API + metrics |
| Metrics | Prometheus | Client lib | Observability |
| Migrations | SQL Scripts | Custom | Schema evolution |
| Config | Environment Variables | `godotenv` | 12-factor app |

---

## Conclusion

This indexer demonstrates **Amazonian principles** of building resilient, scalable, and observable systems:

✅ **Operational Excellence:** Comprehensive metrics, logging, health checks
✅ **Security:** Prepared statements, connection pooling, no SQL injection
✅ **Reliability:** Checkpointing, retry logic, graceful degradation
✅ **Performance Efficiency:** Adaptive parallel processing, connection pooling
✅ **Cost Optimization:** Auto-scaling pipeline based on network conditions

The architecture balances **simplicity** (sequential mode for steady state) with **performance** (parallel mode for catch-up), while maintaining **correctness** through ordered database commits and ACID transactions.
