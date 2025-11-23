# Stellar Indexer - System Design Document

## Executive Summary

This document provides a high-level design (HLD) of the Stellar Indexer system, with special emphasis on the architectural improvements introduced in the `rpc-abstraction` branch compared to `try-again` branch. The system is designed to ingest, process, and index Stellar blockchain data with a focus on USDC transfer events.

---

## Architecture Overview

```mermaid
graph TB
    subgraph "Entry Point"
        MAIN[main.go<br/>CLI Entry Point]
    end

    subgraph "Core Orchestration Layer"
        IDX[Indexer<br/>Main Coordinator]
    end

    subgraph "Service Layer - NEW ARCHITECTURE [NEW]"
        style INGEST_SVC fill:#90EE90
        style RPC_SVC fill:#90EE90

        INGEST_SVC[Ingest Service<br/>OrchestratorService<br/>LOCATION: service/ingest/orchestrator.go]
        RPC_SVC[RPC Service<br/>LedgerBackend<br/>LOCATION: service/rpc/ledger_backend_handler.go]
    end

    subgraph "Integration Layer - NEW [NEW]"
        style RPC_INTEGRATION fill:#FFD700
        RPC_INTEGRATION[RPC Backend Integration<br/>LedgerBuilder<br/>LOCATION: integration/rpc_backend/ledger_builder.go]
    end

    subgraph "Processing Layer"
        PROC[Processors<br/>USDCTransferProcessor]
    end

    subgraph "External Systems"
        STELLAR[Stellar RPC Node<br/>soroban-testnet.stellar.org]
        FUTURE_DB[(Future: MongoDB)]
    end

    MAIN -->|Creates & Configures| IDX
    IDX -->|Initializes| RPC_SVC
    IDX -->|Creates with Processors| INGEST_SVC
    RPC_SVC -->|Uses Builder Pattern| RPC_INTEGRATION
    RPC_INTEGRATION -->|Builds Backend| STELLAR
    INGEST_SVC -->|Fetches Ledgers via| RPC_SVC
    INGEST_SVC -->|Distributes Events to| PROC
    PROC -.->|Future: Persists| FUTURE_DB

    classDef new fill:#90EE90,stroke:#006400,stroke-width:3px
    classDef refactored fill:#FFD700,stroke:#FF8C00,stroke-width:3px
```

---

## Key Architectural Changes: `try-again` → `rpc-abstraction`

### 1. Service Layer Restructuring

```mermaid
flowchart LR
    subgraph "OLD: try-again Branch [OLD]"
        direction TB
        OLD_SERVICES["internal/services/"]
        OLD_INGEST["ingest.go<br/>Monolithic Service"]
        OLD_RPC["rpc_service.go<br/>Direct Backend Access"]

        OLD_SERVICES --> OLD_INGEST
        OLD_SERVICES --> OLD_RPC

        style OLD_SERVICES fill:#FFB6C1,stroke:#DC143C
        style OLD_INGEST fill:#FFB6C1,stroke:#DC143C
        style OLD_RPC fill:#FFB6C1,stroke:#DC143C
    end

    subgraph "NEW: rpc-abstraction Branch [NEW]"
        direction TB
        NEW_SERVICE["internal/service/"]

        subgraph "Ingest Domain"
            NEW_INGEST[service/ingest/<br/>orchestrator.go]
            NEW_INGEST_TYPES[service/ingest/<br/>types.go]
        end

        subgraph "RPC Domain"
            NEW_RPC[service/rpc/<br/>ledger_backend_handler.go]
            NEW_RPC_TYPES[service/rpc/<br/>types.go]
        end

        NEW_SERVICE --> NEW_INGEST
        NEW_SERVICE --> NEW_INGEST_TYPES
        NEW_SERVICE --> NEW_RPC
        NEW_SERVICE --> NEW_RPC_TYPES

        style NEW_SERVICE fill:#90EE90,stroke:#006400
        style NEW_INGEST fill:#90EE90,stroke:#006400
        style NEW_RPC fill:#90EE90,stroke:#006400
    end

    OLD_SERVICES -.->|Refactored to| NEW_SERVICE
```

**Changes:**
- **Renamed**: `internal/services/` → `internal/service/` (singular form)
- **Separated Concerns**: Split monolithic services into domain-specific packages
- **Type Definitions**: Added dedicated `types.go` files for interface contracts
- **Better Modularity**: Each service has clear responsibilities

---

### 2. RPC Backend Abstraction Layer

```mermaid
graph TB
    subgraph "OLD Architecture [OLD]"
        OLD_INGEST_SVC[IngestService]
        OLD_RPC_SVC[RPCService]
        OLD_BACKEND[Direct Backend Creation<br/>ledgerbackend.NewRPCLedgerBackend]

        OLD_INGEST_SVC -->|Tightly Coupled| OLD_RPC_SVC
        OLD_RPC_SVC -->|Creates Directly| OLD_BACKEND

        style OLD_INGEST_SVC fill:#FFB6C1
        style OLD_RPC_SVC fill:#FFB6C1
        style OLD_BACKEND fill:#FFB6C1
    end

    subgraph "NEW Architecture [NEW]"
        NEW_INGEST[OrchestratorService]
        NEW_RPC_HANDLER[LedgerBackend<br/>Handler]
        NEW_INTEGRATION[RPC Integration Layer]
        NEW_BUILDER[LedgerBuilder<br/>Builder Pattern]
        NEW_BACKEND[RPCLedgerBackend]

        NEW_INGEST -->|Uses Interface| RPC_INTERFACE
        RPC_INTERFACE[LedgerBackendHandlerService<br/>Interface]
        RPC_INTERFACE -->|Implemented by| NEW_RPC_HANDLER
        NEW_RPC_HANDLER -->|Delegates to| NEW_INTEGRATION
        NEW_INTEGRATION -->|Uses| NEW_BUILDER
        NEW_BUILDER -->|Builds| NEW_BACKEND

        style NEW_INGEST fill:#90EE90
        style NEW_RPC_HANDLER fill:#90EE90
        style NEW_INTEGRATION fill:#FFD700
        style NEW_BUILDER fill:#FFD700
        style RPC_INTERFACE fill:#87CEEB
    end
```

**Key Improvements:**
- **Interface-Based Design**: `LedgerBackendHandlerService` interface enables testability and flexibility
- **Builder Pattern**: `LedgerBuilder` encapsulates complex RPC backend construction
- **Integration Layer**: New `internal/integration/rpc_backend/` package separates external system integration
- **Lifecycle Management**: Explicit `Start()`, `Close()`, `IsAvailable()` methods

---

### 3. Component Interaction Flow

```mermaid
sequenceDiagram
    participant M as main.go
    participant I as Indexer
    participant RPC as LedgerBackend<br/>(RPC Service)
    participant B as LedgerBuilder<br/>(Integration)
    participant O as OrchestratorService<br/>(Ingest)
    participant P as USDCProcessor
    participant S as Stellar RPC

    Note over M,S: [INIT] INITIALIZATION PHASE

    M->>I: New(config)
    activate I

    I->>RPC: Create LedgerBackend
    activate RPC
    I->>RPC: Start()
    RPC->>B: Build()
    activate B
    B->>B: newBackendOptions()
    B->>S: NewRPCLedgerBackend(options)
    S-->>B: backend instance
    B-->>RPC: backend
    deactivate B
    RPC-->>I: [SUCCESS] Backend Ready
    deactivate RPC

    I->>P: NewUSDCTransferProcessor()
    activate P
    P-->>I: processor

    I->>O: NewIngestService(ledgerBackend, processors)
    activate O
    O-->>I: orchestrator

    I->>I: Start consumeEvents() goroutine
    I-->>M: indexer instance
    deactivate I

    Note over M,S: [INGESTION] INGESTION PHASE

    M->>I: Start(startLedger)
    I->>O: Start(startLedger)
    O->>RPC: PrepareRange(ctx, start, nil)
    RPC->>S: PrepareRange(UnboundedRange)
    S-->>RPC: [SUCCESS] Range Prepared

    O->>O: Start ingestLoop() goroutine

    loop Every 2 seconds
        O->>O: processLedger(sequence)
        O->>RPC: HandleBackend()
        RPC-->>O: backend instance
        O->>S: GetLedger(ctx, sequence)
        S-->>O: LedgerCloseMeta

        O->>P: ProcessLedger(ctx, ledger)
        P-->>O: [SUCCESS]

        O->>S: NewLedgerTransactionReader()
        S-->>O: txReader

        loop For each transaction
            O->>S: Read()
            S-->>O: LedgerTransaction
            O->>P: ProcessTransaction(ctx, tx)
            P->>P: processEvent()
            P->>P: buffer <- USDCTransferEvent
            P-->>O: [SUCCESS]
        end

        O-->>O: Ledger Complete [SUCCESS]
    end

    Note over P,I: [CONSUMPTION] EVENT CONSUMPTION

    loop Continuous
        I->>P: Read from GetBuffer()
        P-->>I: USDCTransferEvent
        I->>I: Log event (Future: Persist to DB)
    end

    deactivate O
    deactivate P
```

---

## Detailed Component Design

### Core Components

#### 1. Indexer (Main Coordinator)
**Location**: `internal/indexer/indexer.go`

**Responsibilities:**
- System initialization and configuration management
- Lifecycle orchestration (startup, shutdown)
- Signal handling (SIGINT, SIGTERM)
- Event consumption coordination

**Key Methods:**
- `New(config Config)`: Initializes all components
- `Start()`: Begins ingestion and blocks until shutdown
- `Stop()`: Graceful shutdown

**Configuration:**
```go
type Config struct {
    RPCEndpoint string  // Stellar RPC node URL
    StartLedger uint32  // Initial ledger sequence
    NetworkPass string  // Network passphrase (testnet/mainnet)
}
```

---

#### 2. OrchestratorService (Ingest Service) [NEW]

**Location**: `internal/service/ingest/orchestrator.go`

**Action Labels:**
- `[COORDINATE]` Ledger ingestion orchestration
- `[DISTRIBUTE]` Event distribution to processors
- `[RESILIENCE]` Error handling with exponential backoff

**Key Features:**
```go
type OrchestratorService struct {
    ledgerBackend rpc.LedgerBackendHandlerService  // Interface-based dependency
    processors    []Processor                       // Pluggable processors
    checkpointMgr CheckpointStore                   // Future: persistence
    ctx           context.Context                   // Lifecycle management
    cancel        context.CancelFunc
    wg            sync.WaitGroup
}
```

**Improvements over old `IngestService`:**
- Interface-based backend dependency (was concrete `*RPCService`)
- Better lifecycle management with context
- Improved error handling (max 5 consecutive errors)
- 2-second polling with configurable ticker

---

#### 3. LedgerBackend (RPC Service) [NEW]

**Location**: `internal/service/rpc/ledger_backend_handler.go`

**Action Labels:**
- `[MANAGE]` Backend lifecycle (Start, Close)
- `[PREPARE]` Ledger range configuration
- `[PROVIDE]` Backend access to consumers

**Interface Contract:**
```go
type LedgerBackendHandlerService interface {
    PrepareRange(ctx context.Context, start, end *uint32) error
    BackendHandlerService[ledgerbackend.LedgerBackend]
}

type BackendHandlerService[T any] interface {
    Start() error
    Close() error
    HandleBackend() (T, error)
    IsAvailable() bool
}
```

**Key Improvements:**
- Generic interface design for extensibility
- Explicit lifecycle states (`isAvailable` flag)
- Separated configuration from instantiation
- Bounded vs Unbounded range support

---

#### 4. RPC Backend Integration Layer [NEW]

**Location**: `internal/integration/rpc_backend/`

**Action Labels:**
- `[BUILD]` RPC backend construction
- `[CONFIGURE]` Client options setup
- `[VALIDATE]` Configuration validation

**Files:**
- `ledger_builder.go`: Builder implementation
- `types.go`: Configuration types and interfaces

**Builder Pattern Implementation:**
```go
type LedgerBuilder struct {
    ClientConfig ClientConfig
}

func (lw *LedgerBuilder) Build() (*ledgerbackend.RPCLedgerBackend, error) {
    // 1. Validate configuration
    // 2. Create backend options
    // 3. Instantiate RPC backend
    // 4. Return configured backend
}
```

**Configuration Structure:**
```go
type ClientConfig struct {
    Endpoint          string              // RPC URL
    BufferSize        int                 // Ledger buffer size
    NetworkPassphrase string              // Network identifier
    TimeoutConfig     ClientTimeoutConfig // Retry/timeout settings
}

type ClientTimeoutConfig struct {
    Timeout  int  // Request timeout (seconds)
    Retries  int  // Max retry attempts
    Interval int  // Retry interval (seconds)
}
```

---

### Processing Pipeline

#### 5. Processor Interface

**Location**: `internal/service/ingest/types.go`

**Action Labels:**
- `[PROCESS]` Ledger and transaction processing
- `[IDENTIFY]` Processor identification

```go
type Processor interface {
    Name() string
    ProcessLedger(ctx context.Context, ledger xdr.LedgerCloseMeta) error
    ProcessTransaction(ctx context.Context, tx ingest.LedgerTransaction) error
}
```

---

#### 6. USDCTransferProcessor

**Location**: `internal/indexer/processors/usdc_transfer_processor.go`

**Action Labels:**
- `[FILTER]` Contract event filtering
- `[PARSE]` Event data extraction
- `[BUFFER]` Event queuing

**Processing Flow:**
```mermaid
graph LR
    A[Transaction] --> B{Has Soroban<br/>Metadata?}
    B -->|No| Z[Skip]
    B -->|Yes| C[Extract Events]
    C --> D{Event Type =<br/>'transfer'?}
    D -->|No| Z
    D -->|Yes| E[Parse Addresses]
    E --> F[Extract Amount]
    F --> G[Create Event]
    G --> H{Buffer<br/>Available?}
    H -->|Yes| I[Queue Event]
    H -->|No| J[Drop Event<br/>Log Warning]
    I --> K[Consumer<br/>Goroutine]
```

**Event Structure:**
```go
type USDCTransferEvent struct {
    Event                      // Base event info
    From   string              // Source address
    To     string              // Destination address
    Amount string              // Transfer amount (i128)
}

type Event struct {
    LedgerSequence uint32
    TxHash         string
    Type           string
    ContractID     string
}
```

---

## Data Flow Architecture

### Ledger Processing Pipeline

```mermaid
flowchart TD
    START([Start Ingestion]) --> PREPARE[PrepareRange<br/>Unbounded from startLedger]
    PREPARE --> LOOP{Ticker<br/>Every 2s}

    LOOP -->|Tick| FETCH[Fetch Ledger N]
    FETCH --> PROC_L[Process Ledger<br/>All Processors]
    PROC_L --> CREATE_TX[Create Transaction Reader]
    CREATE_TX --> TX_LOOP{More<br/>Transactions?}

    TX_LOOP -->|Yes| READ_TX[Read Transaction]
    READ_TX --> PROC_TX[Process Transaction<br/>All Processors]
    PROC_TX --> TX_LOOP

    TX_LOOP -->|No EOF| SUCCESS[SUCCESS Ledger Complete]
    SUCCESS --> INCREMENT[N++]
    INCREMENT --> RESET[Reset Error Counter]
    RESET --> LOOP

    FETCH -->|Error| ERR_COUNT{Error Count<br/>< 5?}
    ERR_COUNT -->|Yes| BACKOFF[Exponential Backoff<br/>Sleep N seconds]
    BACKOFF --> INCREMENT_ERR[Error Count++]
    INCREMENT_ERR --> LOOP

    ERR_COUNT -->|No| STOP([Stop Ingestion])

    LOOP -->|Context Done| GRACEFUL[Graceful Shutdown]
    GRACEFUL --> STOP

    style START fill:#90EE90
    style STOP fill:#FFB6C1
    style SUCCESS fill:#87CEEB
```

---

## Directory Structure Comparison

```
OLD (try-again):                     NEW (rpc-abstraction):
├── internal/                        ├── internal/
│   ├── services/          [OLD]     │   ├── service/           [NEW]
│   │   ├── ingest.go                │   │   ├── ingest/
│   │   └── rpc_service.go           │   │   │   ├── orchestrator.go
│   │                                │   │   │   └── types.go
│   │                                │   │   └── rpc/
│   │                                │   │       ├── ledger_backend_handler.go
│   │                                │   │       └── types.go
│   │                                │   │
│   │                                │   ├── integration/     [NEW]
│   │                                │   │   └── rpc_backend/
│   │                                │   │       ├── ledger_builder.go
│   │                                │   │       └── types.go
│   │                                │   │
│   ├── utils/             [OLD]     │   ├── util/            [RENAMED]
│   │   ├── https_client.go          │   │   ├── https_client.go
│   │   └── utils.go                 │   │   └── utils.go
│   │                                │   │
│   ├── indexer/                     │   ├── indexer/
│   │   └── indexer.go               │   │   ├── indexer.go  (Updated imports)
│   │                                │   │   ├── processors/
│   │                                │   │   └── types/
```

---

## Interface Design Patterns [NEW]

### Generic Backend Handler Pattern

```mermaid
classDiagram
    class BackendHandlerService~T~ {
        <<interface>>
        +Start() error
        +Close() error
        +HandleBackend() (T, error)
        +IsAvailable() bool
    }

    class LedgerBackendHandlerService {
        <<interface>>
        +PrepareRange(ctx, start, end) error
    }

    class LedgerBackend {
        -ClientConfig
        -backend ledgerbackend.LedgerBackend
        -buildErr error
        -isAvailable bool
        +Start() error
        +Close() error
        +HandleBackend() (ledgerbackend.LedgerBackend, error)
        +IsAvailable() bool
        +PrepareRange(ctx, start, end) error
    }

    class BackendBuilder~T~ {
        <<interface>>
        +Build() (*T, error)
    }

    class LedgerBuilder {
        -ClientConfig
        +Build() (*RPCLedgerBackend, error)
        -newBackendOptions() (*RPCLedgerBackendOptions, error)
        -newBackendFromOptions() (*RPCLedgerBackend, error)
    }

    BackendHandlerService <|.. LedgerBackendHandlerService : extends
    LedgerBackendHandlerService <|.. LedgerBackend : implements
    BackendBuilder <|.. LedgerBuilder : implements
    LedgerBackend ..> LedgerBuilder : uses
```

**Benefits:**
- **Type Safety**: Generic interfaces ensure compile-time type checking
- **Testability**: Easy to mock interfaces for unit testing
- **Extensibility**: Support for multiple backend types (RPC, Captive Core)
- **Separation of Concerns**: Clear boundaries between layers

---

## Operational Characteristics

### Resilience Features

| Feature | Implementation | Location |
|---------|---------------|----------|
| **Error Handling** | Exponential backoff (max 5 errors) | `orchestrator.go:73-85` |
| **Graceful Shutdown** | Context cancellation + WaitGroup | `orchestrator.go:153-158` |
| **Signal Handling** | SIGINT/SIGTERM captured | `indexer.go:83-84` |
| **Buffer Overflow** | Non-blocking channel write | `usdc_transfer_processor.go:126-132` |

### Performance Characteristics

```mermaid
graph LR
    subgraph "Ingestion Rate"
        A[2s Poll Interval] --> B["~0.5 ledgers/sec"]
    end

    subgraph "Buffering"
        C[1000 Event Buffer] --> D[Prevents Memory Overflow]
    end

    subgraph "Concurrency"
        E[Goroutine per Consumer] --> F[Async Event Processing]
    end
```

---

## Security & Configuration

### Configuration Management

**Environment-Based Configuration:**
```bash
# .env.example
RPC_ENDPOINT=https://soroban-testnet.stellar.org
START_LEDGER=1696100
NETWORK_PASSPHRASE=Test SDF Network ; September 2015
```

**CLI Flags (Priority Override):**
```bash
./indexer \
  --rpc https://custom-rpc.stellar.org \
  --start 1700000 \
  --network "Test SDF Network ; September 2015"
```

### Security Considerations

- **No Credential Storage**: RPC endpoints are public
- **Read-Only Operations**: System only reads blockchain data
- **Network Isolation**: Future: MongoDB credentials in environment variables
- **Error Information Leakage**: Errors logged but not exposed externally

---

## Future Enhancements

```mermaid
mindmap
    root((Indexer<br/>Roadmap))
        Persistence
            MongoDB Integration
            Checkpoint Management
            Event History Queries
        Scalability
            Horizontal Scaling
            Kafka Event Streaming
            Rate Limiting
        Observability
            Prometheus Metrics
            Distributed Tracing
            Health Checks
        Features
            Multi-Asset Support
            DEX Event Processing
            Real-time Webhooks
```

### Immediate TODOs (from code comments)

1. **Checkpoint Persistence** (`indexer.go:116`)
   - Implement MongoDB storage for processed events
   - Add checkpoint recovery on restart

2. **Latest Ledger Detection** (`main.go:27`)
   - Query RPC for latest ledger when `startLedger=0`
   - Automatic catch-up mechanism

3. **Asset Verification** (`usdc_transfer_processor.go:92`)
   - Proper USDC contract address validation
   - Support for both mainnet and testnet USDC

---

## Deployment Architecture

```mermaid
graph TB
    subgraph "Docker Compose"
        IDX_CONTAINER[Indexer Container]
        MONGO_CONTAINER[(MongoDB Container)]

        IDX_CONTAINER -->|Persists Events| MONGO_CONTAINER
    end

    subgraph "External Services"
        STELLAR_RPC[Stellar RPC<br/>soroban-testnet.stellar.org]
    end

    IDX_CONTAINER -->|Ingests Ledgers| STELLAR_RPC

    USER[Developer] -->|docker-compose up| IDX_CONTAINER
    USER -->|Query Events| MONGO_CONTAINER
```

**Docker Compose Configuration:**
```yaml
# docker-compose.yml
services:
  indexer:
    build: .
    environment:
      - RPC_ENDPOINT=https://soroban-testnet.stellar.org
      - START_LEDGER=1696100
    depends_on:
      - mongodb

  mongodb:
    image: mongo:latest
    ports:
      - "27017:27017"
```

---

## Change Summary: try-again → rpc-abstraction

### Files Modified (13 files)

| Action | File | Lines Changed | Impact |
|--------|------|---------------|--------|
| **ADDED** [NEW] | `integration/rpc_backend/ledger_builder.go` | +46 | Builder pattern for RPC backends |
| **ADDED** [NEW] | `integration/rpc_backend/types.go` | +21 | Configuration types |
| **ADDED** [NEW] | `service/ingest/orchestrator.go` | +158 | Refactored ingest service |
| **ADDED** [NEW] | `service/ingest/types.go` | +21 | Interface definitions |
| **ADDED** [NEW] | `service/rpc/ledger_backend_handler.go` | +79 | RPC lifecycle management |
| **ADDED** [NEW] | `service/rpc/types.go` | +9 | Generic interfaces |
| **DELETED** [OLD] | `services/ingest.go` | -165 | Replaced by orchestrator |
| **DELETED** [OLD] | `services/rpc_service.go` | -83 | Replaced by handler |
| **RENAMED** [RENAMED] | `utils/ → util/` | - | Package naming consistency |
| **MODIFIED** | `indexer/indexer.go` | ~100 | Updated imports and integration |
| **MODIFIED** | `ingest/ingest.go` | +1 | Minor update |
| **MODIFIED** | `.gitignore` | +3 | Added build artifacts |

**Total Impact:**
- **+395 lines added** (new abstractions)
- **-295 lines removed** (old implementations)
- **Net: +100 lines** (with better separation of concerns)

---

## Conclusion

The `rpc-abstraction` branch represents a significant architectural improvement with the following Amazonian principles applied:

### Leadership Principles Demonstrated:

1. **Ownership**: Clear service boundaries and responsibilities
2. **Dive Deep**: Proper abstraction layers without over-engineering
3. **Invent and Simplify**: Builder pattern, generic interfaces
4. **Think Big**: Extensible design for future backend types
5. **Insist on Highest Standards**: Interface-based design, testability

### Key Metrics:

- **Cohesion**: [IMPROVED] Improved (domain-specific packages)
- **Coupling**: [REDUCED] Reduced (interface-based dependencies)
- **Testability**: [IMPROVED] Enhanced (mockable interfaces)
- **Maintainability**: [IMPROVED] Improved (clear separation of concerns)
- **Extensibility**: [IMPROVED] Enhanced (generic patterns)

---

## References

### Code Locations

| Component | Path | Line Reference |
|-----------|------|----------------|
| Ingest Orchestrator | `internal/service/ingest/orchestrator.go` | Full file |
| RPC Handler | `internal/service/rpc/ledger_backend_handler.go` | Full file |
| Ledger Builder | `internal/integration/rpc_backend/ledger_builder.go` | Full file |
| Main Indexer | `internal/indexer/indexer.go` | Full file |
| USDC Processor | `internal/indexer/processors/usdc_transfer_processor.go` | Full file |

### External Dependencies

- **Stellar Go SDK**: `github.com/stellar/go`
- **Go Version**: 1.25.0+
- **Network**: Stellar Testnet (configurable)

---

**Document Version**: 1.0
**Branch**: feat/rpc-abstraction
**Author**: Generated by Claude Code (Amazonian SDE)
**Last Updated**: 2025-11-22