# Stellar Blockchain Indexer - System Design Document

**Author**: System Architecture Review
**Date**: 2025-11-19
**Version**: 1.0
**Status**: Production

---

## Executive Summary

This document presents the High-Level Design (HLD) for a **Stellar Blockchain Indexer** that monitors, extracts, and persists smart contract deployment and activity data from the Stellar Soroban network. The system employs a pattern-based indexing approach focused on factory contract deployments and their child contracts.

---

## 1. System Architecture Overview

```mermaid
graph TB
    subgraph "External Systems"
        SN[Stellar Network<br/>Soroban RPC Server]
        style SN fill:#ff9900,stroke:#232f3e,stroke-width:3px,color:#000
    end

    subgraph "Indexer Application - Go Runtime"
        direction TB

        subgraph "Entry Point"
            MAIN[main.go<br/>Application Bootstrap]
            style MAIN fill:#00a8e1,stroke:#232f3e,stroke-width:2px
        end

        subgraph "Configuration Layer"
            CONFIG[Config Manager<br/>Environment Variables]
            style CONFIG fill:#7aa116,stroke:#232f3e,stroke-width:2px
        end

        subgraph "Ledger Processing Pipeline"
            STREAM[Ledger Streamer<br/>Sequential Polling]
            PROC[Ledger Processor<br/>Transaction Router]
            EXTRACT[Data Extractor<br/>XDR Parser]

            style STREAM fill:#ec7211,stroke:#232f3e,stroke-width:2px
            style PROC fill:#ec7211,stroke:#232f3e,stroke-width:2px
            style EXTRACT fill:#ec7211,stroke:#232f3e,stroke-width:2px
        end

        subgraph "Data Access Layer"
            REPO[Repository Interface]
            PGIMPL[PostgreSQL Implementation<br/>pgx Connection Pool]

            style REPO fill:#146eb4,stroke:#232f3e,stroke-width:2px
            style PGIMPL fill:#146eb4,stroke:#232f3e,stroke-width:2px
        end

        subgraph "Domain Models"
            MODELS[Contract Models<br/>Event Models<br/>Activity Models]
            style MODELS fill:#569a31,stroke:#232f3e,stroke-width:2px
        end
    end

    subgraph "Data Store"
        PG[(PostgreSQL 16<br/>stellar_indexer)]
        style PG fill:#336791,stroke:#232f3e,stroke-width:3px,color:#fff
    end

    subgraph "Monitoring & Observability"
        LOGS[Structured Logging<br/>slog]
        METRICS[Processing Metrics<br/>Ledger Counters]

        style LOGS fill:#879596,stroke:#232f3e,stroke-width:2px
        style METRICS fill:#879596,stroke:#232f3e,stroke-width:2px
    end

    %% Connections with Action Labels
    SN -->|"[ACTION] Stream Ledgers<br/>via JSON-RPC"| STREAM
    MAIN -->|"[ACTION] Load Config"| CONFIG
    MAIN -->|"[ACTION] Initialize"| STREAM
    MAIN -->|"[ACTION] Create Repository"| REPO
    CONFIG -->|"[ACTION] Provide Settings"| STREAM
    CONFIG -->|"[ACTION] Provide DB URL"| REPO

    STREAM -->|"[ACTION] Forward Ledger Data"| PROC
    PROC -->|"[ACTION] Route Transactions"| EXTRACT
    EXTRACT -->|"[ACTION] Parsed Models"| REPO

    REPO -->|"[ACTION] Interface Contract"| PGIMPL
    PGIMPL -->|"[ACTION] Execute SQL<br/>INSERT/SELECT"| PG

    MODELS -.->|"[ACTION] Define Schema"| EXTRACT
    MODELS -.->|"[ACTION] Define Schema"| REPO

    STREAM -->|"[ACTION] Log Events"| LOGS
    PROC -->|"[ACTION] Log Metrics"| METRICS
    EXTRACT -->|"[ACTION] Log Errors"| LOGS

    MAIN -->|"[ACTION] Handle SIGTERM/SIGINT<br/>Graceful Shutdown"| STREAM

    classDef aws fill:#ff9900,stroke:#232f3e,stroke-width:3px,color:#000
    classDef data fill:#336791,stroke:#232f3e,stroke-width:3px,color:#fff
```

---

## 2. Data Flow Architecture

```mermaid
flowchart LR
    subgraph "Source"
        RPC[Stellar RPC<br/>Testnet/Mainnet]
    end

    subgraph "Ingestion Layer"
        POLL[Continuous Polling<br/>RPCLedgerBackend]
        BUFFER[Unbuffered Stream<br/>Sequential Processing]
    end

    subgraph "Processing Layer"
        FILTER{Transaction Filter<br/>Successful + Soroban?}
        ROUTE{Contract Router<br/>Factory or Tracked?}

        subgraph "Factory Path"
            F1[Extract Deployment Data]
            F2[Parse XDR ReturnValue]
            F3[Extract Events/Storage]
            F4[Add to Tracked Map]
        end

        subgraph "Activity Path"
            A1[Extract Activity Data]
            A2[Parse Events/Storage]
            A3[Extract Return Value]
        end
    end

    subgraph "Persistence Layer"
        SAVE[Repository Save Operations]
    end

    subgraph "Storage"
        DB[(PostgreSQL<br/>ACID Transactions)]
    end

    RPC -->|"[ACTION] Fetch Ledger N"| POLL
    POLL -->|"[ACTION] Deserialize"| BUFFER
    BUFFER -->|"[ACTION] Read Txs"| FILTER

    FILTER -->|"[DECISION] Yes"| ROUTE
    FILTER -->|"[DECISION] No - Skip"| BUFFER

    ROUTE -->|"[DECISION] Factory Contract"| F1
    ROUTE -->|"[DECISION] Tracked Contract"| A1
    ROUTE -->|"[DECISION] Untracked - Skip"| BUFFER

    F1 --> F2 --> F3 --> F4
    F4 -->|"[ACTION] Save Contract"| SAVE

    A1 --> A2 --> A3
    A3 -->|"[ACTION] Save Activity"| SAVE

    SAVE -->|"[ACTION] INSERT with JSONB"| DB
    DB -->|"[RESPONSE] Success/Error"| SAVE

    style RPC fill:#ff9900,stroke:#232f3e,stroke-width:2px
    style DB fill:#336791,stroke:#232f3e,stroke-width:2px,color:#fff
    style FILTER fill:#ec7211,stroke:#232f3e,stroke-width:2px
    style ROUTE fill:#ec7211,stroke:#232f3e,stroke-width:2px
```

---

## 3. Component Interaction Diagram

```mermaid
sequenceDiagram
    participant User as Operator
    participant Main as main.go
    participant Config as Config Manager
    participant Stream as Ledger Streamer
    participant Backend as RPC Backend
    participant Proc as Ledger Processor
    participant Extract as Data Extractor
    participant Repo as Repository
    participant DB as PostgreSQL

    User->>+Main: [ACTION] Execute Binary
    Main->>+Config: [ACTION] Load .env
    Config-->>-Main: [RESPONSE] Config Object

    Main->>+DB: [ACTION] Connect (pgx pool)
    DB-->>-Main: [RESPONSE] Connection Pool

    Main->>+Backend: [ACTION] Create RPC Client
    Backend-->>-Main: [RESPONSE] Client Instance

    Main->>+Stream: [ACTION] Initialize Streamer
    Main->>Stream: [ACTION] Start (goroutine)

    loop Continuous Ledger Processing
        Stream->>+Backend: [ACTION] GetLedger(sequence)
        Backend->>Backend: [ACTION] Call Stellar RPC
        Backend-->>-Stream: [RESPONSE] Ledger Data

        Stream->>+Proc: [ACTION] ProcessLedger(ledger)

        loop For Each Transaction
            Proc->>Proc: [DECISION] Is Successful Soroban Tx?
            Proc->>Proc: [ACTION] Extract Contract IDs

            alt Factory Contract Detected
                Proc->>+Extract: [ACTION] ExtractDeploymentData(tx)
                Extract->>Extract: [ACTION] Parse XDR ScVal
                Extract->>Extract: [ACTION] Extract Events
                Extract->>Extract: [ACTION] Extract Storage
                Extract-->>-Proc: [RESPONSE] DeployedContract Model

                Proc->>+Repo: [ACTION] SaveDeployedContract(contract)
                Repo->>+DB: [ACTION] INSERT INTO deployed_contracts
                DB-->>-Repo: [RESPONSE] Success
                Repo->>+DB: [ACTION] INSERT INTO contract_events
                DB-->>-Repo: [RESPONSE] Success
                Repo->>+DB: [ACTION] INSERT INTO storage_entries
                DB-->>-Repo: [RESPONSE] Success
                Repo-->>-Proc: [RESPONSE] Success

                Proc->>Proc: [ACTION] Add to Tracked Map (mutex)
            else Tracked Contract Activity
                Proc->>+Extract: [ACTION] ExtractActivityData(tx)
                Extract->>Extract: [ACTION] Parse Events/Storage
                Extract-->>-Proc: [RESPONSE] ContractActivity Model

                Proc->>+Repo: [ACTION] SaveContractActivity(activity)
                Repo->>+DB: [ACTION] INSERT INTO contract_activities
                DB-->>-Repo: [RESPONSE] Success
                Repo-->>-Proc: [RESPONSE] Success
            end
        end

        Proc-->>-Stream: [RESPONSE] Processed
        Stream->>Stream: [ACTION] Increment Ledger Sequence

        alt Every 10 Ledgers
            Stream->>User: [LOG] Processing Metrics
        end
    end

    User->>Main: [SIGNAL] SIGTERM/SIGINT
    Main->>Stream: [ACTION] Cancel Context
    Stream->>Backend: [ACTION] Close Client
    Main->>DB: [ACTION] Close Pool
    Main->>User: [LOG] Shutdown Complete
```

---

## 4. Database Schema Design

```mermaid
erDiagram
    DEPLOYED_CONTRACTS ||--o{ CONTRACT_EVENTS : "emits"
    DEPLOYED_CONTRACTS ||--o{ STORAGE_ENTRIES : "has"
    DEPLOYED_CONTRACTS ||--o{ CONTRACT_ACTIVITIES : "performs"
    LEDGER_INFO ||--o{ DEPLOYED_CONTRACTS : "contains"

    DEPLOYED_CONTRACTS {
        bigserial id PK "[ACTION] Auto-increment"
        text contract_id UK "[ACTION] Unique identifier"
        text factory_contract_id "[ATTR] Parent factory"
        bigint deployed_ledger "[ATTR] Deployment block"
        timestamp deployed_at "[ACTION] Auto timestamp"
        jsonb init_params "[ATTR] Flexible init data"
        text deployer_role "[ATTR] Contract role"
        bigint cpu_instructions "[METRIC] Resource usage"
        bigint memory_bytes "[METRIC] Resource usage"
        timestamp created_at "[ACTION] Auto on INSERT"
        timestamp updated_at "[ACTION] Auto on UPDATE"
    }

    CONTRACT_EVENTS {
        bigserial id PK "[ACTION] Auto-increment"
        text contract_id "[ATTR] Source contract"
        text tx_hash "[ATTR] Transaction ref"
        bigint ledger "[ATTR] Block number"
        text event_type "[ATTR] Event category"
        text[] topics "[ATTR] Indexed params"
        jsonb data "[ATTR] Event payload"
        timestamp created_at "[ACTION] Auto on INSERT"
    }

    STORAGE_ENTRIES {
        bigserial id PK "[ACTION] Auto-increment"
        text contract_id "[ATTR] Owner contract"
        text tx_hash "[ATTR] Transaction ref"
        bigint ledger "[ATTR] Block number"
        text storage_key "[ATTR] State key"
        text storage_value "[ATTR] Current value"
        text change_type "[ATTR] created/updated/removed"
        text previous_value "[ATTR] Audit trail"
        bytea key_bytes "[ATTR] Raw XDR key"
        bytea value_bytes "[ATTR] Raw XDR value"
        timestamp created_at "[ACTION] Auto on INSERT"
    }

    CONTRACT_ACTIVITIES {
        bigserial id PK "[ACTION] Auto-increment"
        text contract_id "[ATTR] Target contract"
        text tx_hash UK "[ATTR] Unique tx hash"
        bigint ledger "[ATTR] Block number"
        text function_name "[ATTR] Invoked function"
        jsonb parameters "[ATTR] Function args"
        jsonb return_value "[ATTR] Function result"
        jsonb events "[ATTR] Emitted events"
        jsonb storage_changes "[ATTR] State diffs"
        bigint cpu_instructions "[METRIC] Resource usage"
        bigint memory_bytes "[METRIC] Resource usage"
        timestamp created_at "[ACTION] Auto on INSERT"
    }

    LEDGER_INFO {
        bigint ledger PK "[ATTR] Ledger sequence"
        timestamp processed_at "[ACTION] Processing time"
        int transaction_count "[METRIC] Tx count"
    }
```

### Database Indexes Strategy

```mermaid
graph LR
    subgraph "Index Types"
        BTREE[B-Tree Indexes<br/>Fast Lookups]
        GIN[GIN Indexes<br/>JSONB Queries]
    end

    subgraph "B-Tree Indexed Columns"
        B1[contract_id]
        B2[tx_hash]
        B3[ledger]
        B4[deployed_ledger]
        B5[created_at]
    end

    subgraph "GIN Indexed Columns"
        G1[init_params JSONB]
        G2[data JSONB]
        G3[parameters JSONB]
        G4[events JSONB]
    end

    BTREE --> B1 & B2 & B3 & B4 & B5
    GIN --> G1 & G2 & G3 & G4

    B1 & B2 & B3 & B4 & B5 -->|"[ACTION] Enable Fast<br/>Equality/Range Queries"| PERF1[Query Performance]
    G1 & G2 & G3 & G4 -->|"[ACTION] Enable JSONB<br/>Containment Queries"| PERF2[Flexible Filtering]

    style BTREE fill:#146eb4,stroke:#232f3e,stroke-width:2px
    style GIN fill:#ec7211,stroke:#232f3e,stroke-width:2px
    style PERF1 fill:#7aa116,stroke:#232f3e,stroke-width:2px
    style PERF2 fill:#7aa116,stroke:#232f3e,stroke-width:2px
```

---

## 5. Deployment Architecture

```mermaid
graph TB
    subgraph "Development Environment"
        DEV[Developer Machine]

        subgraph "Docker Compose Stack"
            PG_DOCKER[PostgreSQL 16 Alpine<br/>Port: 5433→5432]
            PGADMIN[pgAdmin<br/>Port: 5050]
        end

        subgraph "Indexer Process"
            BINARY[./bin/indexer<br/>Compiled Binary]
        end
    end

    subgraph "External Dependencies"
        TESTNET[Stellar Testnet<br/>soroban-testnet.stellar.org]
        MAINNET[Stellar Mainnet<br/>soroban-mainnet.stellar.org]
    end

    subgraph "Configuration Files"
        ENV[.env File<br/>Environment Variables]
        SCHEMA[schema.sql<br/>Database DDL]
    end

    DEV -->|"[ACTION] make db-up"| PG_DOCKER
    DEV -->|"[ACTION] make build"| BINARY
    ENV -->|"[ACTION] Load Config"| BINARY
    SCHEMA -->|"[ACTION] Initialize Schema"| PG_DOCKER

    BINARY -->|"[ACTION] Connect pgx Pool"| PG_DOCKER
    BINARY -->|"[ACTION] Stream Ledgers<br/>HTTPS JSON-RPC"| TESTNET
    BINARY -->|"[ACTION] Stream Ledgers<br/>HTTPS JSON-RPC"| MAINNET

    DEV -->|"[ACTION] Browse DB<br/>localhost:5050"| PGADMIN
    PGADMIN -->|"[ACTION] Query Tables"| PG_DOCKER

    style PG_DOCKER fill:#336791,stroke:#232f3e,stroke-width:2px,color:#fff
    style TESTNET fill:#ff9900,stroke:#232f3e,stroke-width:2px
    style MAINNET fill:#ff9900,stroke:#232f3e,stroke-width:2px
    style BINARY fill:#ec7211,stroke:#232f3e,stroke-width:2px
```

---

## 6. Concurrency & Thread Safety Model

```mermaid
graph TB
    subgraph "Main Goroutine"
        MAIN_G[Main Thread]
        SIGNAL[Signal Handler<br/>SIGTERM/SIGINT]
    end

    subgraph "Streaming Goroutine"
        STREAM_G[Ledger Streamer<br/>Long-running Worker]
        CTX[Context with Cancel]
    end

    subgraph "Shared State - Thread Safe"
        TRACKED[Tracked Contracts Map<br/>sync.RWMutex]
        PGPOOL[pgx Connection Pool<br/>Thread-safe]
    end

    subgraph "Processing Guarantees"
        SEQ[Sequential Ledger Processing<br/>No Parallel Ledgers]
        ATOMIC[Atomic DB Transactions<br/>ACID Guarantees]
    end

    MAIN_G -->|"[ACTION] Spawn"| STREAM_G
    MAIN_G -->|"[ACTION] Create"| CTX
    MAIN_G -->|"[ACTION] Listen Signals"| SIGNAL

    STREAM_G -->|"[ACTION] Read Context"| CTX
    STREAM_G -->|"[ACTION] RLock/Lock"| TRACKED
    STREAM_G -->|"[ACTION] Acquire Connection"| PGPOOL

    SIGNAL -->|"[ACTION] Cancel"| CTX
    CTX -->|"[ACTION] Stop Streaming"| STREAM_G

    STREAM_G -->|"[GUARANTEE] Sequential Order"| SEQ
    PGPOOL -->|"[GUARANTEE] Transaction Isolation"| ATOMIC

    style TRACKED fill:#ec7211,stroke:#232f3e,stroke-width:2px
    style PGPOOL fill:#146eb4,stroke:#232f3e,stroke-width:2px
    style SEQ fill:#7aa116,stroke:#232f3e,stroke-width:2px
    style ATOMIC fill:#7aa116,stroke:#232f3e,stroke-width:2px
```

---

## 7. Error Handling & Resilience

```mermaid
flowchart TD
    START[Application Start]

    START --> INIT_CHECK{Initialization<br/>Successful?}

    INIT_CHECK -->|"[ERROR] DB Connection Failed"| LOG_FATAL[Log Fatal & Exit]
    INIT_CHECK -->|"[ERROR] RPC Client Failed"| LOG_FATAL
    INIT_CHECK -->|"[SUCCESS]"| STREAM_LOOP

    STREAM_LOOP[Stream Ledgers Loop]

    STREAM_LOOP --> FETCH{Fetch Ledger}

    FETCH -->|"[ERROR] RPC Error"| LOG_ERROR[Log Error & Retry]
    FETCH -->|"[SUCCESS]"| PROCESS

    LOG_ERROR --> BACKOFF[Exponential Backoff]
    BACKOFF --> FETCH

    PROCESS[Process Transactions]
    PROCESS --> EXTRACT{Extract Data}

    EXTRACT -->|"[ERROR] XDR Parse Error"| LOG_WARN[Log Warning & Skip Tx]
    EXTRACT -->|"[SUCCESS]"| SAVE

    LOG_WARN --> NEXT_TX[Next Transaction]
    NEXT_TX --> STREAM_LOOP

    SAVE[Save to Database]
    SAVE --> DB_OP{Database Operation}

    DB_OP -->|"[ERROR] Constraint Violation"| LOG_ERROR_DB[Log Error & Skip]
    DB_OP -->|"[ERROR] Connection Lost"| RECONNECT[Reconnect Pool]
    DB_OP -->|"[SUCCESS]"| NEXT_LEDGER[Next Ledger]

    LOG_ERROR_DB --> NEXT_TX
    RECONNECT --> SAVE
    NEXT_LEDGER --> STREAM_LOOP

    STREAM_LOOP --> SHUTDOWN_SIGNAL{Shutdown Signal?}
    SHUTDOWN_SIGNAL -->|"[SIGNAL] Received"| GRACEFUL[Graceful Shutdown]
    SHUTDOWN_SIGNAL -->|"[CONTINUE]"| STREAM_LOOP

    GRACEFUL --> CLOSE_CONN[Close DB Connections]
    CLOSE_CONN --> CLOSE_CLIENT[Close RPC Client]
    CLOSE_CLIENT --> EXIT[Exit 0]

    style LOG_FATAL fill:#d13212,stroke:#232f3e,stroke-width:2px,color:#fff
    style LOG_ERROR fill:#ff9900,stroke:#232f3e,stroke-width:2px
    style LOG_WARN fill:#ffbb00,stroke:#232f3e,stroke-width:2px
    style GRACEFUL fill:#7aa116,stroke:#232f3e,stroke-width:2px
```

---

## 8. Observability & Monitoring Strategy

```mermaid
graph LR
    subgraph "Application Instrumentation"
        APP[Indexer Application]
    end

    subgraph "Logging - Structured slog"
        DEBUG[DEBUG Level<br/>Every Ledger Processed]
        INFO[INFO Level<br/>Every 10 Ledgers + Metrics]
        WARN[WARN Level<br/>Parse Errors]
        ERROR[ERROR Level<br/>DB/RPC Failures]
    end

    subgraph "Metrics - In-Process"
        LEDGER_COUNT[Ledger Counter]
        FETCH_TIME[Fetch Duration]
        PROCESS_TIME[Processing Duration]
        TX_COUNT[Transaction Count]
    end

    subgraph "Database Observability"
        PG_LOGS[PostgreSQL Logs]
        PGADMIN_DASH[pgAdmin Dashboards]
        TABLE_STATS[Table Row Counts]
    end

    subgraph "Future Enhancements"
        PROMETHEUS[Prometheus Metrics Export]
        GRAFANA[Grafana Dashboards]
        ALERTS[Alerting Rules]
    end

    APP -->|"[ACTION] Log Structured Events"| DEBUG
    APP -->|"[ACTION] Log Structured Events"| INFO
    APP -->|"[ACTION] Log Structured Events"| WARN
    APP -->|"[ACTION] Log Structured Events"| ERROR

    APP -->|"[ACTION] Track Metrics"| LEDGER_COUNT
    APP -->|"[ACTION] Track Metrics"| FETCH_TIME
    APP -->|"[ACTION] Track Metrics"| PROCESS_TIME
    APP -->|"[ACTION] Track Metrics"| TX_COUNT

    APP -->|"[ACTION] Execute Queries"| PG_LOGS
    PG_LOGS --> PGADMIN_DASH
    PGADMIN_DASH --> TABLE_STATS

    DEBUG & INFO & WARN & ERROR -.->|"[FUTURE] Export"| PROMETHEUS
    LEDGER_COUNT & FETCH_TIME & PROCESS_TIME & TX_COUNT -.->|"[FUTURE] Export"| PROMETHEUS
    PROMETHEUS -.->|"[FUTURE] Visualize"| GRAFANA
    GRAFANA -.->|"[FUTURE] Trigger"| ALERTS

    style PROMETHEUS fill:#e6522c,stroke:#232f3e,stroke-width:2px,stroke-dasharray: 5 5
    style GRAFANA fill:#f46800,stroke:#232f3e,stroke-width:2px,stroke-dasharray: 5 5
    style ALERTS fill:#d13212,stroke:#232f3e,stroke-width:2px,stroke-dasharray: 5 5,color:#fff
```

---

## 9. Security Considerations

```mermaid
graph TB
    subgraph "Configuration Security"
        ENV_FILE[.env File<br/>Sensitive Credentials]
        GIT_IGNORE[.gitignore<br/>Exclude .env]
    end

    subgraph "Database Security"
        DB_CREDS[Database Credentials<br/>Strong Password]
        CONN_POOL[Connection Pool<br/>Max Connections: 25]
        SSL_MODE[SSL Mode: prefer]
    end

    subgraph "Network Security"
        RPC_TLS[HTTPS Only<br/>TLS 1.2+]
        LOCALHOST[PostgreSQL Bind<br/>localhost only]
    end

    subgraph "Application Security"
        INPUT_VAL[XDR Validation<br/>Stellar SDK]
        ERROR_HANDLING[No Sensitive Data in Logs]
        GRACEFUL_SD[Graceful Shutdown<br/>No Data Loss]
    end

    subgraph "Threats Mitigated"
        T1[Credential Exposure]
        T2[SQL Injection]
        T3[Connection Exhaustion]
        T4[MITM Attacks]
        T5[Data Corruption]
    end

    ENV_FILE -->|"[CONTROL] Git Exclusion"| GIT_IGNORE
    GIT_IGNORE -->|"[MITIGATION]"| T1

    DB_CREDS -->|"[CONTROL] Env Variable"| ENV_FILE
    CONN_POOL -->|"[CONTROL] Limit Connections"| T3
    CONN_POOL -->|"[CONTROL] Prepared Statements"| T2

    RPC_TLS -->|"[CONTROL] Encrypted Transport"| T4
    LOCALHOST -->|"[CONTROL] No External Access"| T4

    INPUT_VAL -->|"[CONTROL] Schema Validation"| T5
    GRACEFUL_SD -->|"[CONTROL] Clean Shutdown"| T5

    style T1 fill:#d13212,stroke:#232f3e,stroke-width:2px,color:#fff
    style T2 fill:#d13212,stroke:#232f3e,stroke-width:2px,color:#fff
    style T3 fill:#d13212,stroke:#232f3e,stroke-width:2px,color:#fff
    style T4 fill:#d13212,stroke:#232f3e,stroke-width:2px,color:#fff
    style T5 fill:#d13212,stroke:#232f3e,stroke-width:2px,color:#fff
```

---

## 10. Scalability & Performance Optimization

```mermaid
graph TB
    subgraph "Current Bottlenecks"
        SEQ[Sequential Ledger Processing<br/>Single-threaded]
        SINGLE_FACTORY[Single Factory Monitoring<br/>Limited Scope]
    end

    subgraph "Optimization Opportunities"
        BATCH[Batch DB Inserts<br/>Reduce Round-trips]
        WORKER_POOL[Worker Pool Pattern<br/>Parallel Tx Processing]
        CACHE[In-memory Cache<br/>Recent Storage State]
        MULTI_FACTORY[Multi-factory Support<br/>Configurable List]
    end

    subgraph "Database Optimizations"
        PARTITIONING[Table Partitioning<br/>By Ledger Range]
        MATERIALIZED_VIEWS[Materialized Views<br/>Aggregated Stats]
        INDEX_TUNING[Index Optimization<br/>Query Analysis]
    end

    subgraph "Infrastructure Scaling"
        HORIZONTAL[Horizontal Scaling<br/>Multiple Indexers]
        READ_REPLICA[Read Replicas<br/>Query Offloading]
        SHARDING[Ledger Range Sharding<br/>Parallel Processing]
    end

    subgraph "Performance Targets"
        TARGET1[Throughput: 100 ledgers/sec]
        TARGET2[Latency: < 1s per ledger]
        TARGET3[Availability: 99.9%]
    end

    SEQ -->|"[OPTIMIZATION] Implement"| WORKER_POOL
    SEQ -->|"[OPTIMIZATION] Implement"| BATCH
    SINGLE_FACTORY -->|"[OPTIMIZATION] Extend"| MULTI_FACTORY

    WORKER_POOL -->|"[ACTION] Enable"| TARGET1
    BATCH -->|"[ACTION] Reduce"| TARGET2

    PARTITIONING -->|"[ACTION] Improve Query Speed"| INDEX_TUNING
    MATERIALIZED_VIEWS -->|"[ACTION] Precompute Stats"| READ_REPLICA

    HORIZONTAL -->|"[ACTION] Achieve"| TARGET3
    SHARDING -->|"[ACTION] Improve"| TARGET1
    READ_REPLICA -->|"[ACTION] Reduce Load"| TARGET2

    CACHE -.->|"[FUTURE]"| TARGET2

    style SEQ fill:#ff9900,stroke:#232f3e,stroke-width:2px
    style TARGET1 fill:#7aa116,stroke:#232f3e,stroke-width:2px
    style TARGET2 fill:#7aa116,stroke:#232f3e,stroke-width:2px
    style TARGET3 fill:#7aa116,stroke:#232f3e,stroke-width:2px
```

---

## 11. Technology Stack Summary

| Layer | Technology | Purpose | Action |
|-------|-----------|---------|--------|
| **Language** | Go 1.25 | System Programming | [COMPILE] High-performance binary |
| **Blockchain SDK** | stellar/go | Stellar Integration | [PARSE] XDR, Ledger data |
| **Database Driver** | pgx/v5 | PostgreSQL Client | [EXECUTE] Pooled connections |
| **Database** | PostgreSQL 16 | Persistent Storage | [STORE] ACID transactions |
| **Configuration** | godotenv | Environment Management | [LOAD] .env variables |
| **Logging** | slog | Structured Logging | [LOG] JSON events |
| **Container** | Docker Compose | Local Development | [ORCHESTRATE] Multi-service |
| **Build Tool** | Make | Build Automation | [BUILD] Compile & Run |

---

## 12. Key Design Decisions & Trade-offs

### ✅ Design Decisions

1. **Sequential Ledger Processing**
   - **Decision**: Process ledgers one at a time in order
   - **Rationale**: Ensures data consistency and prevents race conditions
   - **Trade-off**: Limits throughput but guarantees correctness

2. **Pattern-based Indexing**
   - **Decision**: Only index factory contracts and their children
   - **Rationale**: Reduces storage and processing overhead
   - **Trade-off**: Not a full blockchain indexer, purpose-built

3. **JSONB for Flexible Data**
   - **Decision**: Use JSONB columns for events, parameters, init data
   - **Rationale**: Schema flexibility for evolving contract interfaces
   - **Trade-off**: Query performance vs. schema rigidity

4. **Thread-safe Tracked Contracts**
   - **Decision**: Use sync.RWMutex for in-memory tracking
   - **Rationale**: Fast lookups without DB queries
   - **Trade-off**: State lost on restart (future: persist checkpoints)

5. **Connection Pooling**
   - **Decision**: pgx pool with 25 max connections
   - **Rationale**: Efficient database resource usage
   - **Trade-off**: Connection limits vs. parallelism

### 🎯 Operational Excellence Principles

```mermaid
mindmap
  root((Operational<br/>Excellence))
    Reliability
      Graceful Shutdown
      Error Recovery
      ACID Transactions
    Performance
      Connection Pooling
      Indexed Queries
      Sequential Consistency
    Observability
      Structured Logging
      Processing Metrics
      Database Stats
    Security
      Credential Management
      TLS Encryption
      Input Validation
    Maintainability
      Clean Architecture
      Interface Abstraction
      Comprehensive Docs
```

---

## 13. Future Enhancements Roadmap

```mermaid
gantt
    title Stellar Indexer Enhancement Roadmap
    dateFormat YYYY-MM-DD

    section Phase 1 - Foundation
    REST API for Queries                :done, p1-1, 2025-01-01, 30d
    Checkpoint Persistence             :done, p1-2, 2025-01-15, 20d
    Multi-factory Support              :active, p1-3, 2025-02-01, 15d

    section Phase 2 - Performance
    Batch Database Inserts             :p2-1, 2025-02-15, 20d
    Worker Pool for Parallel Processing :p2-2, 2025-03-01, 30d
    In-memory Caching                  :p2-3, 2025-03-15, 20d

    section Phase 3 - Observability
    Prometheus Metrics Export          :p3-1, 2025-04-01, 15d
    Grafana Dashboards                 :p3-2, 2025-04-10, 10d
    Alerting Rules                     :p3-3, 2025-04-15, 10d

    section Phase 4 - Scale
    Table Partitioning                 :p4-1, 2025-05-01, 20d
    Read Replicas                      :p4-2, 2025-05-15, 15d
    Horizontal Scaling                 :p4-3, 2025-06-01, 30d
```

---

## Appendix A: Configuration Reference

### Environment Variables

```bash
# Stellar Network Configuration
RPC_SERVER_URL=https://soroban-testnet.stellar.org
NETWORK_PASSPHRASE=Test SDF Network ; September 2015

# Indexer Settings
START_LEDGER=0  # 0 = auto-detect (latest - 10)
BUFFER_SIZE=1000

# Factory Contracts to Monitor
FACTORY_CONTRACT_IDs=CDTXVQ...

# Logging
LOG_LEVEL=info  # debug, info, warn, error

# Database
DATABASE_URL=postgresql://indexer:password@localhost:5433/stellar_indexer
```

---

## Appendix B: Database Queries Examples

### Query: Get all deployed contracts by factory
```sql
SELECT contract_id, deployed_ledger, deployer_role, created_at
FROM deployed_contracts
WHERE factory_contract_id = 'CDTXVQ...'
ORDER BY deployed_ledger DESC;
```

### Query: Get contract activity timeline
```sql
SELECT ca.ledger, ca.function_name, ca.parameters, ca.return_value
FROM contract_activities ca
WHERE ca.contract_id = 'CCXY123...'
ORDER BY ca.ledger ASC;
```

### Query: Get latest storage state
```sql
SELECT contract_id, storage_key, storage_value
FROM latest_storage_state
WHERE contract_id = 'CCXY123...';
```

---

## Appendix C: Monitoring Checklist

- [ ] Monitor ledger processing lag (current ledger vs. network latest)
- [ ] Track database connection pool utilization
- [ ] Monitor RPC request success rate
- [ ] Alert on parsing errors exceeding threshold
- [ ] Track average ledger processing time
- [ ] Monitor PostgreSQL disk usage
- [ ] Alert on indexer process downtime
- [ ] Track tracked contracts map size
- [ ] Monitor database query performance

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-11-19 | Architecture Team | Initial HLD creation |

---

**End of System Design Document**

*This document represents the current state of the Stellar Blockchain Indexer architecture. For implementation details, refer to the codebase in `/home/aaj/work/trustless/Indexer`.*
