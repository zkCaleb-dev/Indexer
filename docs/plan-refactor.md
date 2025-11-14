# Guía de Implementación: Indexador de Escrows Stellar

**Documento maestro con instrucciones procedurales completas**

Este documento contiene toda la arquitectura, código y pasos necesarios para implementar un indexador de contratos escrow en Stellar usando Soroban RPC, PostgreSQL y Go.

---

## Tabla de Contenidos

1. [Arquitectura General](#arquitectura-general)
2. [Stack Tecnológico](#stack-tecnológico)
3. [Estructura de Carpetas](#estructura-de-carpetas)
4. [Esquema de Base de Datos](#esquema-de-base-de-datos)
5. [Paso 1: Setup del Proyecto](#paso-1-setup-del-proyecto)
6. [Paso 2: Configuración](#paso-2-configuración)
7. [Paso 3: Cliente Soroban RPC](#paso-3-cliente-soroban-rpc)
8. [Paso 4: Decoders de Eventos](#paso-4-decoders-de-eventos)
9. [Paso 5: Store Layer (PostgreSQL)](#paso-5-store-layer-postgresql)
10. [Paso 6: Indexer Core](#paso-6-indexer-core)
11. [Paso 7: API REST](#paso-7-api-rest)
12. [Paso 8: Deployment en Railway](#paso-8-deployment-en-railway)
13. [Anexo: Tipos de Datos](#anexo-tipos-de-datos)

---

## Arquitectura General

```
┌─────────────────────────────────────────────────────────────┐
│                    APLICACIÓN GOLANG                        │
│                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐ │
│  │   Soroban    │───▶│   Event      │───▶│  PostgreSQL  │ │
│  │   RPC Poll   │    │  Processor   │    │   Writer     │ │
│  │              │    │              │    │              │ │
│  │ • getEvents  │    │ • Decode     │    │ • Batch      │ │
│  │ • Filters    │    │ • Validate   │    │ • UPSERT     │ │
│  │ • Checkpoint │    │ • Transform  │    │ • Checkpoint │ │
│  └──────────────┘    └──────────────┘    └──────────────┘ │
│         ↑                                        ↓          │
│         │                                        │          │
│         └────────── Checkpoint Recovery ─────────┘          │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                    REST API                            │ │
│  │  • GET /escrow/{id}          (estado actual)          │ │
│  │  • GET /escrow/{id}/history  (eventos históricos)     │ │
│  │  • GET /escrows?filters      (queries complejas)      │ │
│  │  • GET /dashboard/stats      (agregaciones)           │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              ↓
                    ┌──────────────────┐
                    │   PostgreSQL     │
                    │                  │
                    │  • escrows       │ ← Estado actual
                    │  • milestones    │ ← Milestones mutables
                    │  • deposits      │ ← Transferencias (futuro)
                    │  • events        │ ← Histórico completo
                    │  • checkpoints   │ ← Recovery
                    └──────────────────┘
```

**Características clave:**
- Monolito modular (todo en un proceso)
- Polling con Soroban RPC `getEvents` (eficiente para contratos específicos)
- Esquema híbrido: relacional para estado + JSONB para histórico
- Transacciones atómicas para consistencia
- Checkpoints para recovery automático

---

## Stack Tecnológico

### Core
```go
// go.mod
module github.com/Trustless-Work/Indexer

go 1.21

require (
    github.com/stellar/go v0.0.0-20241101000000-xxxxxxxxxxxx
    github.com/jackc/pgx/v5 v5.5.1
    github.com/go-chi/chi/v5 v5.0.11
    github.com/rs/zerolog v1.31.0
    github.com/spf13/viper v1.18.2
    github.com/pkg/errors v0.9.1
)
```

### Infraestructura
- **Database:** PostgreSQL 15+ en Railway
- **Hosting:** Railway Pro ($20/mes)
- **Network:** Stellar Testnet/Mainnet
- **Monitoring:** Railway Grafana (incluido)

---

## Estructura de Carpetas

```
stellar-escrow-indexer/
├── cmd/
│   └── indexer/
│       └── main.go                     # Entry point único
│
├── internal/
│   ├── config/
│   │   └── config.go                   # Viper config
│   │
│   ├── stellar/
│   │   ├── clients.go                  # Horizon + RPC clients
│   │   └── rpc_client.go               # JSON-RPC implementation
│   │
│   ├── events/
│   │   ├── decoder.go                  # Base XDR decoder
│   │   ├── escrow_decoder.go           # Escrow events decoder
│   │   └── transfer_decoder.go         # SAC transfer decoder
│   │
│   ├── indexer/
│   │   ├── service.go                  # Main indexer service
│   │   ├── processor.go                # Event processor
│   │   ├── buffer.go                   # In-memory buffer
│   │   └── checkpoint.go               # Checkpoint manager
│   │
│   ├── store/
│   │   ├── postgres.go                 # pgx pool setup
│   │   ├── escrows.go                  # Escrows repository
│   │   ├── milestones.go               # Milestones repository
│   │   ├── events.go                   # Events repository
│   │   ├── deposits.go                 # Deposits repository (futuro)
│   │   └── checkpoint.go               # Checkpoint repository
│   │
│   └── api/
│       ├── server.go                   # Chi router setup
│       ├── handlers.go                 # HTTP handlers
│       ├── middleware.go               # Logging, CORS, recovery
│       └── dto.go                      # Response DTOs
│
├── pkg/
│   └── types/
│       ├── escrow.go                   # Escrow domain types
│       ├── events.go                   # Event types
│       └── deposit.go                  # Deposit types
│
├── migrations/
│   ├── 001_initial_schema.sql
│   └── 002_add_indexes.sql
│
├── configs/
│   ├── config.yaml                     # Development
│   └── config.prod.yaml                # Production
│
├── scripts/
│   └── migrate.sh                      # Database migrations
│
├── Dockerfile
├── docker-compose.yml
├── .env.example
└── README.md
```

---

## Esquema de Base de Datos

### SQL Completo (migrations/001_initial_schema.sql)

```sql
-- ============================================
-- TABLA 1: Estado actual de escrows
-- ============================================
CREATE TABLE escrows (
    id TEXT PRIMARY KEY,                    -- Contract ID
    engagement_id TEXT NOT NULL,
    contract_type TEXT NOT NULL CHECK (contract_type IN ('single-release', 'multi-release')),
    status TEXT NOT NULL DEFAULT 'pending',
    
    -- Metadata
    title TEXT NOT NULL,
    description TEXT,
    receiver_memo TEXT DEFAULT '0',
    
    -- Financiero
    platform_fee INTEGER NOT NULL,          -- Basis points (500 = 5%)
    
    -- SINGLE-RELEASE específico (NULL si multi-release)
    amount NUMERIC,                         -- Total amount para single-release
    receiver TEXT,                          -- Receiver para single-release
    
    -- Roles (se consultan frecuentemente)
    approver TEXT NOT NULL,
    service_provider TEXT NOT NULL,
    platform_address TEXT NOT NULL,
    release_signer TEXT NOT NULL,
    dispute_resolver TEXT NOT NULL,
    
    -- Flags globales
    is_disputed BOOLEAN DEFAULT FALSE,
    is_released BOOLEAN DEFAULT FALSE,
    is_resolved BOOLEAN DEFAULT FALSE,
    
    -- Trustline
    trustline_address TEXT NOT NULL,
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    
    -- Validación condicional
    CONSTRAINT single_release_fields CHECK (
        (contract_type = 'single-release' AND amount IS NOT NULL AND receiver IS NOT NULL)
        OR
        (contract_type = 'multi-release' AND amount IS NULL AND receiver IS NULL)
    )
);

-- ============================================
-- TABLA 2: Milestones (estado mutable)
-- ============================================
CREATE TABLE milestones (
    id BIGSERIAL PRIMARY KEY,
    escrow_id TEXT NOT NULL REFERENCES escrows(id) ON DELETE CASCADE,
    milestone_index INTEGER NOT NULL,
    
    -- Contenido
    description TEXT NOT NULL,
    evidence TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    
    -- SINGLE-RELEASE específico
    approved BOOLEAN,
    
    -- MULTI-RELEASE específico
    amount NUMERIC,
    receiver TEXT,
    is_released BOOLEAN,
    is_disputed BOOLEAN,
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    
    UNIQUE(escrow_id, milestone_index)
);

-- ============================================
-- TABLA 3: Deposits (transferencias SAC)
-- ============================================
CREATE TABLE deposits (
    id TEXT PRIMARY KEY,                    -- tx_hash + ledger
    escrow_id TEXT,                         -- NULL si no está asociado aún
    
    -- Transfer data
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    amount NUMERIC NOT NULL,
    asset TEXT NOT NULL,                    -- 'USDC' | 'EURC'
    
    -- Blockchain metadata
    ledger_sequence BIGINT NOT NULL,
    transaction_hash TEXT NOT NULL,
    contract_id TEXT NOT NULL,              -- Token contract ID
    
    -- Status
    processed BOOLEAN DEFAULT FALSE,
    
    -- Timestamps
    occurred_at TIMESTAMPTZ NOT NULL,
    indexed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    FOREIGN KEY (escrow_id) REFERENCES escrows(id)
);

-- ============================================
-- TABLA 4: Eventos históricos (append-only)
-- ============================================
CREATE TABLE events (
    id BIGSERIAL PRIMARY KEY,
    escrow_id TEXT NOT NULL,
    
    event_type TEXT NOT NULL,               -- 'tw_init' | 'tw_fund' | 'tw_release' | etc.
    event_data JSONB NOT NULL,              -- Payload COMPLETO
    
    -- Blockchain metadata
    ledger_sequence BIGINT NOT NULL,
    transaction_hash TEXT NOT NULL,
    contract_id TEXT NOT NULL,
    
    -- Timestamps
    occurred_at TIMESTAMPTZ NOT NULL,
    indexed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    FOREIGN KEY (escrow_id) REFERENCES escrows(id),
    UNIQUE(transaction_hash, event_type, escrow_id)
);

-- ============================================
-- TABLA 5: Checkpoint para recovery
-- ============================================
CREATE TABLE indexer_checkpoint (
    id INT PRIMARY KEY DEFAULT 1,
    last_cursor TEXT NOT NULL,
    last_ledger_seq BIGINT NOT NULL,
    last_processed_at TIMESTAMPTZ NOT NULL,
    
    CHECK (id = 1)
);

-- Insertar checkpoint inicial
INSERT INTO indexer_checkpoint (id, last_cursor, last_ledger_seq, last_processed_at)
VALUES (1, '', 0, NOW());
```

### Índices (migrations/002_add_indexes.sql)

```sql
-- Escrows
CREATE INDEX idx_escrows_status ON escrows(status);
CREATE INDEX idx_escrows_type ON escrows(contract_type);
CREATE INDEX idx_escrows_created_at ON escrows(created_at DESC);
CREATE INDEX idx_escrows_approver ON escrows(approver);
CREATE INDEX idx_escrows_service_provider ON escrows(service_provider);
CREATE INDEX idx_escrows_type_status ON escrows(contract_type, status);

-- Milestones
CREATE INDEX idx_milestones_escrow ON milestones(escrow_id);
CREATE INDEX idx_milestones_status ON milestones(status);
CREATE INDEX idx_milestones_escrow_status ON milestones(escrow_id, status);

-- Deposits
CREATE INDEX idx_deposits_escrow ON deposits(escrow_id);
CREATE INDEX idx_deposits_to_address ON deposits(to_address);
CREATE INDEX idx_deposits_ledger ON deposits(ledger_sequence);
CREATE INDEX idx_deposits_processed ON deposits(processed) WHERE processed = false;

-- Events
CREATE INDEX idx_events_escrow_id ON events(escrow_id);
CREATE INDEX idx_events_occurred_at ON events(occurred_at DESC);
CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_ledger ON events(ledger_sequence);
CREATE INDEX idx_events_escrow_time ON events(escrow_id, occurred_at DESC);
CREATE INDEX idx_events_data_gin ON events USING GIN(event_data jsonb_path_ops);
```

---

## Paso 1: Setup del Proyecto

### 1.1 Inicializar proyecto Go

```bash
mkdir stellar-escrow-indexer
cd stellar-escrow-indexer
go mod init github.com/Trustless-Work/Indexer

# Instalar dependencias
go get github.com/stellar/go@latest
go get github.com/jackc/pgx/v5@v5.5.1
go get github.com/go-chi/chi/v5@v5.0.11
go get github.com/rs/zerolog@v1.31.0
go get github.com/spf13/viper@v1.18.2
go get github.com/pkg/errors@v0.9.1
```

### 1.2 Crear estructura de carpetas

```bash
mkdir -p cmd/indexer
mkdir -p internal/{config,stellar,events,indexer,store,api}
mkdir -p pkg/types
mkdir -p migrations
mkdir -p configs
mkdir -p scripts
```

### 1.3 Configurar PostgreSQL local (docker-compose.yml)

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: escrow_indexer
      POSTGRES_USER: indexer
      POSTGRES_PASSWORD: indexer_pass
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d

volumes:
  postgres_data:
```

```bash
# Iniciar PostgreSQL
docker-compose up -d

# Verificar
psql -h localhost -U indexer -d escrow_indexer -c "\dt"
```

---

## Paso 2: Configuración

### internal/config/config.go

```go
package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type Config struct {
	// Network
	Network         string `mapstructure:"network"`
	HorizonURL      string `mapstructure:"horizon_url"`
	SorobanRPCURL   string `mapstructure:"soroban_rpc_url"`
	
	// Contracts
	EscrowContractIDs    []string `mapstructure:"escrow_contract_ids"`
	USDCTokenContractID  string   `mapstructure:"usdc_token_contract_id"`
	EURCTokenContractID  string   `mapstructure:"eurc_token_contract_id"`
	
	// Database
	DatabaseURL     string `mapstructure:"database_url"`
	
	// Indexer
	PollInterval    int    `mapstructure:"poll_interval_seconds"`
	BatchSize       int    `mapstructure:"batch_size"`
	StartLedger     uint32 `mapstructure:"start_ledger"`
	
	// API
	APIPort         string `mapstructure:"api_port"`
	
	// Logging
	LogLevel        string `mapstructure:"log_level"`
}

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()
	
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	return &cfg, nil
}

func (c *Config) IsTestnet() bool {
	return c.Network == "testnet"
}
```

### configs/config.yaml

```yaml
# Network
network: testnet
horizon_url: https://horizon-testnet.stellar.org
soroban_rpc_url: https://soroban-testnet.stellar.org

# Contracts (actualizar con tus contract IDs)
escrow_contract_ids:
  - CDQJ...XXXX  # Single-release contract
  - CDZP...YYYY  # Multi-release contract
usdc_token_contract_id: CBIELTK6YBZJU5UP2WWQEUCYKLPU6AUNZ2BQ4WWFEIE3USCIHMXQDAMA
eurc_token_contract_id: CBQHNAXSI55GX2GN6D67GK7BHVPSLJUGZQEU7WJ5LKR5PNUCGLIMAO4K

# Database
database_url: postgres://indexer:indexer_pass@localhost:5432/escrow_indexer?sslmode=disable

# Indexer
poll_interval_seconds: 5
batch_size: 100
start_ledger: 0  # 0 = continuar desde checkpoint

# API
api_port: 8080

# Logging
log_level: info
```

---

## Paso 3: Cliente Soroban RPC

### internal/stellar/rpc_client.go

```go
package stellar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	
	"github.com/stellar/go/xdr"
)

type RPCClient struct {
	url        string
	httpClient *http.Client
}

func NewRPCClient(url string) *RPCClient {
	return &RPCClient{
		url: url,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ========== Request/Response Types ==========

type GetEventsRequest struct {
	StartLedger uint32           `json:"startLedger,omitempty"`
	Cursor      string           `json:"cursor,omitempty"`
	Filters     []EventFilter    `json:"filters"`
	Pagination  *PaginationParam `json:"pagination,omitempty"`
}

type EventFilter struct {
	Type        string     `json:"type"`
	ContractIDs []string   `json:"contractIds,omitempty"`
	Topics      [][]string `json:"topics,omitempty"`
}

type PaginationParam struct {
	Limit int `json:"limit,omitempty"`
}

type GetEventsResponse struct {
	Events       []SorobanEvent `json:"events"`
	LatestLedger uint32         `json:"latestLedger"`
	OldestLedger uint32         `json:"oldestLedger"`
	Cursor       string         `json:"cursor,omitempty"`
}

type SorobanEvent struct {
	Type           string   `json:"type"`
	Ledger         uint32   `json:"ledger"`
	LedgerClosedAt string   `json:"ledgerClosedAt"`
	ContractID     string   `json:"contractId"`
	ID             string   `json:"id"`
	PagingToken    string   `json:"pagingToken,omitempty"`
	Topics         []string `json:"topic"` // API usa "topic" singular
	Value          string   `json:"value"`
	InSuccessfulContractCall bool `json:"inSuccessfulContractCall"`
	TxHash         string   `json:"txHash,omitempty"`
}

type RPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type LatestLedgerResponse struct {
	Sequence        uint32 `json:"sequence"`
	Hash            string `json:"hash"`
	ProtocolVersion uint32 `json:"protocolVersion"`
}

// ========== Methods ==========

func (c *RPCClient) GetEvents(ctx context.Context, req GetEventsRequest) (*GetEventsResponse, error) {
	rpcReq := RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getEvents",
		Params:  req,
	}
	
	reqBody, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}
	
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	var rpcResp RPCResponse
	if err := json.Unmarshal(bodyBytes, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}
	
	resultBytes, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	
	var eventsResp GetEventsResponse
	if err := json.Unmarshal(resultBytes, &eventsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal events response: %w", err)
	}
	
	return &eventsResp, nil
}

func (c *RPCClient) GetLatestLedger(ctx context.Context) (*LatestLedgerResponse, error) {
	rpcReq := RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "getLatestLedger",
	}
	
	reqBody, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
	}
	
	resultBytes, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	
	var latestResp LatestLedgerResponse
	if err := json.Unmarshal(resultBytes, &latestResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal latest ledger response: %w", err)
	}
	
	return &latestResp, nil
}

// ========== Helper: Decode Topic ==========

func DecodeTopicToEventName(topic string) string {
	var scVal xdr.ScVal
	err := xdr.SafeUnmarshalBase64(topic, &scVal)
	if err != nil {
		return "unknown"
	}
	
	if scVal.Type == xdr.ScValTypeScvSymbol {
		sym := scVal.Sym
		if sym != nil {
			return string(*sym)
		}
	}
	
	return "unknown"
}
```

### internal/stellar/clients.go

```go
package stellar

import (
	"context"
	"fmt"
	"net/http"
	"time"
	
	"github.com/Trustless-Work/Indexer/internal/config"
	"github.com/pkg/errors"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/xdr"
)

type Clients struct {
	Horizon    *horizonclient.Client
	RPC        *RPCClient
	config     *config.Config
	httpClient *http.Client
}

func NewClients(cfg *config.Config) (*Clients, error) {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	horizonClient := &horizonclient.Client{
		HorizonURL: cfg.HorizonURL,
		HTTP:       httpClient,
	}
	
	rpcClient := NewRPCClient(cfg.SorobanRPCURL)
	
	return &Clients{
		Horizon:    horizonClient,
		RPC:        rpcClient,
		config:     cfg,
		httpClient: httpClient,
	}, nil
}

// ========== Event Filter Builders ==========

// BuildEscrowEventFilter construye filtro para eventos de escrow
func (c *Clients) BuildEscrowEventFilter(contractID string, eventSymbol string) EventFilter {
	symbolBase64, err := c.symbolToBase64(eventSymbol)
	if err != nil {
		fmt.Printf("Warning: failed to encode event symbol %s: %v\n", eventSymbol, err)
		symbolBase64 = "*"
	}
	
	filter := EventFilter{
		Type:   "contract",
		Topics: [][]string{{symbolBase64}},
	}
	
	if contractID != "" {
		filter.ContractIDs = []string{contractID}
	}
	
	return filter
}

func (c *Clients) BuildEscrowInitFilter(contractID string) EventFilter {
	return c.BuildEscrowEventFilter(contractID, "tw_init")
}

func (c *Clients) BuildEscrowFundFilter(contractID string) EventFilter {
	return c.BuildEscrowEventFilter(contractID, "tw_fund")
}

func (c *Clients) BuildMilestoneApprovalFilter(contractID string) EventFilter {
	return c.BuildEscrowEventFilter(contractID, "tw_ms_approve")
}

func (c *Clients) BuildEscrowReleaseFilter(contractID string) EventFilter {
	return c.BuildEscrowEventFilter(contractID, "tw_release")
}

func (c *Clients) BuildEscrowDisputeFilter(contractID string) EventFilter {
	return c.BuildEscrowEventFilter(contractID, "tw_dispute")
}

func (c *Clients) BuildDisputeResolveFilter(contractID string) EventFilter {
	return c.BuildEscrowEventFilter(contractID, "tw_disp_resolve")
}

func (c *Clients) BuildMilestoneStatusChangeFilter(contractID string) EventFilter {
	return c.BuildEscrowEventFilter(contractID, "tw_ms_change")
}

func (c *Clients) BuildEscrowUpdateFilter(contractID string) EventFilter {
	return c.BuildEscrowEventFilter(contractID, "tw_update")
}

func (c *Clients) BuildTTLExtendFilter(contractID string) EventFilter {
	return c.BuildEscrowEventFilter(contractID, "tw_ttl_extend")
}

// BuildTransferFilter construye filtro para transferencias SAC
func (c *Clients) BuildTransferFilter(toAddress string, tokenContractID string) EventFilter {
	toAddrBase64, err := c.contractAddressToBase64(toAddress)
	if err != nil {
		toAddrBase64 = "*"
	}
	
	const transferSymbol = "AAAADwAAAAh0cmFuc2Zlcg==" // "transfer"
	
	return EventFilter{
		Type:        "contract",
		ContractIDs: []string{tokenContractID},
		Topics:      [][]string{{transferSymbol, "*", toAddrBase64, "*"}},
	}
}

// ========== Helpers ==========

func (c *Clients) symbolToBase64(symbol string) (string, error) {
	scSymbol := xdr.ScSymbol(symbol)
	scVal := xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &scSymbol,
	}
	
	base64Str, err := xdr.MarshalBase64(scVal)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal symbol %s", symbol)
	}
	
	return base64Str, nil
}

func (c *Clients) contractAddressToBase64(contractAddr string) (string, error) {
	contractHash, err := strkey.Decode(strkey.VersionByteContract, contractAddr)
	if err != nil {
		return "", errors.Wrapf(err, "failed to decode contract address %s", contractAddr)
	}
	
	var hash xdr.Hash
	copy(hash[:], contractHash)
	
	contractID := xdr.ContractId(hash)
	scAddress := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractID,
	}
	
	scVal := xdr.ScVal{
		Type:    xdr.ScValTypeScvAddress,
		Address: &scAddress,
	}
	
	base64Str, err := xdr.MarshalBase64(scVal)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal contract address %s", contractAddr)
	}
	
	return base64Str, nil
}

func (c *Clients) GetLatestLedgerFromRPC(ctx context.Context) (uint32, error) {
	resp, err := c.RPC.GetLatestLedger(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get latest ledger from RPC")
	}
	return resp.Sequence, nil
}
```

---

## Paso 4: Decoders de Eventos

### internal/events/decoder.go (Base decoder)

```go
package events

import (
	"math/big"
	
	"github.com/pkg/errors"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/xdr"
)

type Decoder struct{}

func NewDecoder() *Decoder {
	return &Decoder{}
}

// decodeSymbol decodifica un ScVal symbol
func (d *Decoder) decodeSymbol(base64Str string) (string, error) {
	var scVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(base64Str, &scVal); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal symbol")
	}
	
	sym, ok := scVal.GetSym()
	if !ok {
		return "", errors.Errorf("ScVal is not a symbol, got type: %v", scVal.Type)
	}
	
	return string(sym), nil
}

// decodeAddress decodifica un ScVal address
func (d *Decoder) decodeAddress(base64Str string) (string, error) {
	var scVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(base64Str, &scVal); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal address")
	}
	
	addr, ok := scVal.GetAddress()
	if !ok {
		return "", errors.Errorf("ScVal is not an address, got type: %v", scVal.Type)
	}
	
	return d.scAddressToString(addr)
}

// decodeI128 decodifica un ScVal i128
func (d *Decoder) decodeI128(base64Str string) (string, error) {
	var scVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(base64Str, &scVal); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal i128")
	}
	
	i128Val, ok := scVal.GetI128()
	if !ok {
		return "", errors.Errorf("ScVal is not an i128, got type: %v", scVal.Type)
	}
	
	hi := big.NewInt(int64(i128Val.Hi))
	lo := new(big.Int).SetUint64(uint64(i128Val.Lo))
	
	result := new(big.Int).Lsh(hi, 64)
	result.Add(result, lo)
	
	return result.String(), nil
}

func (d *Decoder) decodeU32(base64Str string) (uint32, error) {
	var scVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(base64Str, &scVal); err != nil {
		return 0, errors.Wrap(err, "failed to unmarshal u32")
	}
	
	u32Val, ok := scVal.GetU32()
	if !ok {
		return 0, errors.Errorf("ScVal is not a u32, got type: %v", scVal.Type)
	}
	
	return u32Val, nil
}

func (d *Decoder) decodeString(base64Str string) (string, error) {
	var scVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(base64Str, &scVal); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal string")
	}
	
	str, ok := scVal.GetStr()
	if !ok {
		return "", errors.Errorf("ScVal is not a string, got type: %v", scVal.Type)
	}
	
	return string(str), nil
}

func (d *Decoder) scAddressToString(addr xdr.ScAddress) (string, error) {
	switch addr.Type {
	case xdr.ScAddressTypeScAddressTypeAccount:
		accountID, ok := addr.GetAccountId()
		if !ok {
			return "", errors.New("failed to get account ID")
		}
		return accountID.Address(), nil
		
	case xdr.ScAddressTypeScAddressTypeContract:
		contractHash, ok := addr.GetContractId()
		if !ok {
			return "", errors.New("failed to get contract ID")
		}
		encoded, err := strkey.Encode(strkey.VersionByteContract, contractHash[:])
		if err != nil {
			return "", errors.Wrap(err, "failed to encode contract hash")
		}
		return encoded, nil
		
	default:
		return "", errors.Errorf("unknown ScAddress type: %v", addr.Type)
	}
}
```

### internal/events/escrow_decoder.go

```go
package events

import (
	"fmt"
	
	"github.com/Trustless-Work/Indexer/internal/stellar"
	"github.com/Trustless-Work/Indexer/pkg/types"
	"github.com/stellar/go/xdr"
)

type EscrowDecoder struct {
	base *Decoder
}

func NewEscrowDecoder() *EscrowDecoder {
	return &EscrowDecoder{
		base: NewDecoder(),
	}
}

// DecodeTwInit decodifica un evento tw_init
func (d *EscrowDecoder) DecodeTwInit(event stellar.SorobanEvent) (*types.EscrowInitEvent, error) {
	var scVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(event.Value, &scVal); err != nil {
		return nil, fmt.Errorf("failed to decode tw_init value: %w", err)
	}
	
	escrowMap, ok := scVal.GetMap()
	if !ok {
		return nil, fmt.Errorf("expected Map for tw_init, got %v", scVal.Type)
	}
	
	initEvent := &types.EscrowInitEvent{
		Ledger:     event.Ledger,
		TxHash:     event.TxHash,
		ContractID: event.ContractID,
		OccurredAt: event.LedgerClosedAt,
	}
	
	// Parsear campos del mapa
	for _, entry := range *escrowMap {
		key, _ := entry.Key.GetSym()
		
		switch string(key) {
		case "engagement_id":
			str, _ := entry.Val.GetStr()
			initEvent.EngagementID = string(str)
			
		case "contract_type":
			sym, _ := entry.Val.GetSym()
			initEvent.ContractType = string(sym)
			
		case "title":
			str, _ := entry.Val.GetStr()
			initEvent.Title = string(str)
			
		case "description":
			str, _ := entry.Val.GetStr()
			initEvent.Description = string(str)
			
		case "amount":
			if i128, ok := entry.Val.GetI128(); ok {
				initEvent.Amount = d.i128ToString(i128)
			}
			
		case "platform_fee":
			if u32, ok := entry.Val.GetU32(); ok {
				initEvent.PlatformFee = int32(u32)
			}
			
		case "receiver_memo":
			str, _ := entry.Val.GetStr()
			initEvent.ReceiverMemo = string(str)
			
		case "roles":
			if rolesMap, ok := entry.Val.GetMap(); ok {
				initEvent.Roles = d.parseRoles(*rolesMap)
			}
			
		case "milestones":
			if vec, ok := entry.Val.GetVec(); ok {
				initEvent.Milestones = d.parseMilestones(*vec)
			}
			
		case "trustline":
			if trustlineMap, ok := entry.Val.GetMap(); ok {
				initEvent.Trustline = d.parseTrustline(*trustlineMap)
			}
		}
	}
	
	return initEvent, nil
}

// DecodeTwMsApprove decodifica un evento tw_ms_approve
func (d *EscrowDecoder) DecodeTwMsApprove(event stellar.SorobanEvent) (*types.MilestoneApproveEvent, error) {
	var scVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(event.Value, &scVal); err != nil {
		return nil, fmt.Errorf("failed to decode tw_ms_approve: %w", err)
	}
	
	mapVal, ok := scVal.GetMap()
	if !ok {
		return nil, fmt.Errorf("expected Map for tw_ms_approve")
	}
	
	var milestoneIndex uint32
	for _, entry := range *mapVal {
		key, _ := entry.Key.GetSym()
		if string(key) == "milestone_index" {
			milestoneIndex, _ = entry.Val.GetU32()
		}
	}
	
	return &types.MilestoneApproveEvent{
		ContractID:     event.ContractID,
		MilestoneIndex: milestoneIndex,
		Ledger:         event.Ledger,
		TxHash:         event.TxHash,
		OccurredAt:     event.LedgerClosedAt,
	}, nil
}

// DecodeTwRelease decodifica un evento tw_release
func (d *EscrowDecoder) DecodeTwRelease(event stellar.SorobanEvent) (*types.EscrowReleaseEvent, error) {
	var scVal xdr.ScVal
	if err := xdr.SafeUnmarshalBase64(event.Value, &scVal); err != nil {
		return nil, fmt.Errorf("failed to decode tw_release: %w", err)
	}
	
	releaseEvent := &types.EscrowReleaseEvent{
		ContractID: event.ContractID,
		Ledger:     event.Ledger,
		TxHash:     event.TxHash,
		OccurredAt: event.LedgerClosedAt,
	}
	
	// Si el value es un mapa, puede tener milestone_index (multi-release)
	if mapVal, ok := scVal.GetMap(); ok {
		for _, entry := range *mapVal {
			key, _ := entry.Key.GetSym()
			if string(key) == "milestone_index" {
				if u32Val, ok := entry.Val.GetU32(); ok {
					releaseEvent.MilestoneIndex = &u32Val
				}
			}
		}
	}
	
	return releaseEvent, nil
}

// DecodeTwFund decodifica un evento tw_fund
func (d *EscrowDecoder) DecodeTwFund(event stellar.SorobanEvent) (*types.EscrowFundEvent, error) {
	return &types.EscrowFundEvent{
		ContractID: event.ContractID,
		Ledger:     event.Ledger,
		TxHash:     event.TxHash,
		OccurredAt: event.LedgerClosedAt,
	}, nil
}

// DecodeTwDispute decodifica un evento tw_dispute
func (d *EscrowDecoder) DecodeTwDispute(event stellar.SorobanEvent) (*types.EscrowDisputeEvent, error) {
	return &types.EscrowDisputeEvent{
		ContractID: event.ContractID,
		Ledger:     event.Ledger,
		TxHash:     event.TxHash,
		OccurredAt: event.LedgerClosedAt,
	}, nil
}

// ========== Helpers ==========

func (d *EscrowDecoder) i128ToString(i128 xdr.Int128Parts) string {
	// Usar lógica del decoder base
	// (implementación simplificada aquí)
	return fmt.Sprintf("%d", i128.Lo) // TODO: implementar correctamente
}

func (d *EscrowDecoder) parseRoles(rolesMap xdr.ScMap) types.EscrowRoles {
	roles := types.EscrowRoles{}
	
	for _, entry := range rolesMap {
		key, _ := entry.Key.GetSym()
		addr, _ := entry.Val.GetAddress()
		addrStr, _ := d.base.scAddressToString(addr)
		
		switch string(key) {
		case "approver":
			roles.Approver = addrStr
		case "service_provider":
			roles.ServiceProvider = addrStr
		case "platform_address":
			roles.PlatformAddress = addrStr
		case "release_signer":
			roles.ReleaseSigner = addrStr
		case "dispute_resolver":
			roles.DisputeResolver = addrStr
		case "receiver":
			roles.Receiver = addrStr
		}
	}
	
	return roles
}

func (d *EscrowDecoder) parseMilestones(vec *xdr.ScVec) []types.MilestoneInit {
	milestones := make([]types.MilestoneInit, 0, len(*vec))
	
	for _, msVal := range *vec {
		msMap, ok := msVal.GetMap()
		if !ok {
			continue
		}
		
		ms := types.MilestoneInit{}
		
		for _, entry := range *msMap {
			key, _ := entry.Key.GetSym()
			
			switch string(key) {
			case "description":
				str, _ := entry.Val.GetStr()
				ms.Description = string(str)
			case "evidence":
				str, _ := entry.Val.GetStr()
				ms.Evidence = string(str)
			case "status":
				str, _ := entry.Val.GetStr()
				ms.Status = string(str)
			case "approved":
				if b, ok := entry.Val.GetB(); ok {
					ms.Approved = &b
				}
			case "amount":
				if i128, ok := entry.Val.GetI128(); ok {
					amountStr := d.i128ToString(i128)
					ms.Amount = &amountStr
				}
			case "receiver":
				if addr, ok := entry.Val.GetAddress(); ok {
					addrStr, _ := d.base.scAddressToString(addr)
					ms.Receiver = &addrStr
				}
			}
		}
		
		milestones = append(milestones, ms)
	}
	
	return milestones
}

func (d *EscrowDecoder) parseTrustline(trustlineMap xdr.ScMap) types.TrustlineInfo {
	trustline := types.TrustlineInfo{}
	
	for _, entry := range trustlineMap {
		key, _ := entry.Key.GetSym()
		if string(key) == "address" {
			addr, _ := entry.Val.GetAddress()
			addrStr, _ := d.base.scAddressToString(addr)
			trustline.Address = addrStr
		}
	}
	
	return trustline
}
```

### internal/events/transfer_decoder.go

```go
package events

import (
	"fmt"
	
	"github.com/Trustless-Work/Indexer/internal/stellar"
	"github.com/Trustless-Work/Indexer/pkg/types"
	"github.com/pkg/errors"
)

type TransferDecoder struct {
	base *Decoder
}

func NewTransferDecoder() *TransferDecoder {
	return &TransferDecoder{
		base: NewDecoder(),
	}
}

// DecodeTransferEvent decodifica un evento SAC transfer
func (d *TransferDecoder) DecodeTransferEvent(event stellar.SorobanEvent, asset string) (*types.DepositEntry, error) {
	if len(event.Topics) < 3 {
		return nil, errors.Errorf("event has %d topics, expected 3 for SAC transfer", len(event.Topics))
	}
	
	// Topics: [symbol, from, to]
	symbol, err := d.base.decodeSymbol(event.Topics[0])
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode symbol")
	}
	if symbol != "transfer" {
		return nil, errors.Errorf("expected 'transfer' symbol, got '%s'", symbol)
	}
	
	fromAddr, err := d.base.decodeAddress(event.Topics[1])
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode from address")
	}
	
	toAddr, err := d.base.decodeAddress(event.Topics[2])
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode to address")
	}
	
	amount, err := d.base.decodeI128(event.Value)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode amount")
	}
	
	return &types.DepositEntry{
		ID:              fmt.Sprintf("%s-%d", event.TxHash, event.Ledger),
		FromAddress:     fromAddr,
		ToAddress:       toAddr,
		Amount:          amount,
		Asset:           asset,
		LedgerSequence:  int64(event.Ledger),
		TransactionHash: event.TxHash,
		ContractID:      event.ContractID,
		OccurredAt:      event.LedgerClosedAt,
		Processed:       false,
	}, nil
}
```

---

## Paso 5: Store Layer (PostgreSQL)

### internal/store/postgres.go

```go
package store

import (
	"context"
	"fmt"
	"runtime"
	"time"
	
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}
	
	// Optimizar pool
	config.MaxConns = int32(runtime.NumCPU() * 2)
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute
	
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}
	
	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}
```

### internal/store/escrows.go

```go
package store

import (
	"context"
	"fmt"
	
	"github.com/Trustless-Work/Indexer/pkg/types"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateEscrow(ctx context.Context, tx pgx.Tx, event *types.EscrowInitEvent) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO escrows (
			id, engagement_id, contract_type, status,
			title, description, platform_fee,
			amount, receiver,
			approver, service_provider, platform_address,
			release_signer, dispute_resolver,
			trustline_address, receiver_memo,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`,
		event.ContractID,
		event.EngagementID,
		event.ContractType,
		"pending",
		event.Title,
		event.Description,
		event.PlatformFee,
		event.Amount,
		event.Roles.Receiver,
		event.Roles.Approver,
		event.Roles.ServiceProvider,
		event.Roles.PlatformAddress,
		event.Roles.ReleaseSigner,
		event.Roles.DisputeResolver,
		event.Trustline.Address,
		event.ReceiverMemo,
		event.OccurredAt,
		event.OccurredAt,
	)
	
	return err
}

func (s *Store) UpdateEscrowStatus(ctx context.Context, tx pgx.Tx, contractID string, status string, occurredAt string) error {
	_, err := tx.Exec(ctx, `
		UPDATE escrows
		SET status = $2, updated_at = $3
		WHERE id = $1
	`, contractID, status, occurredAt)
	
	return err
}

func (s *Store) MarkEscrowReleased(ctx context.Context, tx pgx.Tx, contractID string, occurredAt string) error {
	_, err := tx.Exec(ctx, `
		UPDATE escrows
		SET is_released = true, status = 'released', updated_at = $2
		WHERE id = $1
	`, contractID, occurredAt)
	
	return err
}

func (s *Store) MarkEscrowDisputed(ctx context.Context, tx pgx.Tx, contractID string, occurredAt string) error {
	_, err := tx.Exec(ctx, `
		UPDATE escrows
		SET is_disputed = true, updated_at = $2
		WHERE id = $1
	`, contractID, occurredAt)
	
	return err
}

func (s *Store) GetEscrow(ctx context.Context, contractID string) (*types.Escrow, error) {
	var escrow types.Escrow
	
	err := s.pool.QueryRow(ctx, `
		SELECT id, engagement_id, contract_type, status,
			   title, description, receiver_memo,
			   platform_fee, amount, receiver,
			   approver, service_provider, platform_address,
			   release_signer, dispute_resolver,
			   is_disputed, is_released, is_resolved,
			   trustline_address,
			   created_at, updated_at
		FROM escrows
		WHERE id = $1
	`, contractID).Scan(
		&escrow.ID, &escrow.EngagementID, &escrow.ContractType, &escrow.Status,
		&escrow.Title, &escrow.Description, &escrow.ReceiverMemo,
		&escrow.PlatformFee, &escrow.Amount, &escrow.Receiver,
		&escrow.Approver, &escrow.ServiceProvider, &escrow.PlatformAddress,
		&escrow.ReleaseSigner, &escrow.DisputeResolver,
		&escrow.IsDisputed, &escrow.IsReleased, &escrow.IsResolved,
		&escrow.TrustlineAddress,
		&escrow.CreatedAt, &escrow.UpdatedAt,
	)
	
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("escrow not found: %s", contractID)
	}
	
	return &escrow, err
}
```

### internal/store/milestones.go

```go
package store

import (
	"context"
	
	"github.com/Trustless-Work/Indexer/pkg/types"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateMilestones(ctx context.Context, tx pgx.Tx, contractID string, milestones []types.MilestoneInit, occurredAt string) error {
	for i, ms := range milestones {
		_, err := tx.Exec(ctx, `
			INSERT INTO milestones (
				escrow_id, milestone_index,
				description, evidence, status,
				approved, amount, receiver,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`,
			contractID,
			i,
			ms.Description,
			ms.Evidence,
			ms.Status,
			ms.Approved,
			ms.Amount,
			ms.Receiver,
			occurredAt,
			occurredAt,
		)
		
		if err != nil {
			return err
		}
	}
	
	return nil
}

func (s *Store) ApproveMilestone(ctx context.Context, tx pgx.Tx, contractID string, milestoneIndex uint32, occurredAt string) error {
	_, err := tx.Exec(ctx, `
		UPDATE milestones
		SET approved = true, status = 'approved', updated_at = $3
		WHERE escrow_id = $1 AND milestone_index = $2
	`, contractID, milestoneIndex, occurredAt)
	
	return err
}

func (s *Store) ReleaseMilestone(ctx context.Context, tx pgx.Tx, contractID string, milestoneIndex uint32, occurredAt string) error {
	_, err := tx.Exec(ctx, `
		UPDATE milestones
		SET is_released = true, status = 'released', updated_at = $3
		WHERE escrow_id = $1 AND milestone_index = $2
	`, contractID, milestoneIndex, occurredAt)
	
	return err
}

func (s *Store) CheckAllMilestonesApproved(ctx context.Context, tx pgx.Tx, contractID string) (bool, error) {
	var allApproved bool
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*) = COUNT(*) FILTER (WHERE approved = true)
		FROM milestones
		WHERE escrow_id = $1
	`, contractID).Scan(&allApproved)
	
	return allApproved, err
}

func (s *Store) CheckAllMilestonesReleased(ctx context.Context, tx pgx.Tx, contractID string) (bool, error) {
	var allReleased bool
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*) = COUNT(*) FILTER (WHERE is_released = true)
		FROM milestones
		WHERE escrow_id = $1
	`, contractID).Scan(&allReleased)
	
	return allReleased, err
}
```

### internal/store/events.go

```go
package store

import (
	"context"
	"encoding/json"
	
	"github.com/jackc/pgx/v5"
)

func (s *Store) InsertEvent(ctx context.Context, tx pgx.Tx, contractID string, eventType string, eventData interface{}, ledger uint32, txHash string, occurredAt string) error {
	dataJSON, err := json.Marshal(eventData)
	if err != nil {
		return err
	}
	
	_, err = tx.Exec(ctx, `
		INSERT INTO events (
			escrow_id, event_type, event_data,
			ledger_sequence, transaction_hash, contract_id,
			occurred_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (transaction_hash, event_type, escrow_id) DO NOTHING
	`,
		contractID,
		eventType,
		dataJSON,
		ledger,
		txHash,
		contractID,
		occurredAt,
	)
	
	return err
}
```

### internal/store/checkpoint.go

```go
package store

import (
	"context"
	"time"
	
	"github.com/jackc/pgx/v5"
)

type Checkpoint struct {
	LastCursor       string
	LastLedgerSeq    uint32
	LastProcessedAt  time.Time
}

func (s *Store) GetCheckpoint(ctx context.Context) (*Checkpoint, error) {
	var cp Checkpoint
	
	err := s.pool.QueryRow(ctx, `
		SELECT last_cursor, last_ledger_seq, last_processed_at
		FROM indexer_checkpoint
		WHERE id = 1
	`).Scan(&cp.LastCursor, &cp.LastLedgerSeq, &cp.LastProcessedAt)
	
	if err == pgx.ErrNoRows {
		return &Checkpoint{LastLedgerSeq: 0}, nil
	}
	
	return &cp, err
}

func (s *Store) UpdateCheckpoint(ctx context.Context, tx pgx.Tx, cursor string, ledgerSeq uint32) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO indexer_checkpoint (id, last_cursor, last_ledger_seq, last_processed_at)
		VALUES (1, $1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			last_cursor = EXCLUDED.last_cursor,
			last_ledger_seq = EXCLUDED.last_ledger_seq,
			last_processed_at = EXCLUDED.last_processed_at
	`, cursor, ledgerSeq, time.Now())
	
	return err
}
```

---

## Paso 6: Indexer Core

### internal/indexer/processor.go

```go
package indexer

import (
	"context"
	"fmt"
	
	"github.com/Trustless-Work/Indexer/internal/events"
	"github.com/Trustless-Work/Indexer/internal/stellar"
	"github.com/Trustless-Work/Indexer/internal/store"
	"github.com/rs/zerolog/log"
)

type EventProcessor struct {
	store          *store.Store
	escrowDecoder  *events.EscrowDecoder
	transferDecoder *events.TransferDecoder
}

func NewEventProcessor(store *store.Store) *EventProcessor {
	return &EventProcessor{
		store:           store,
		escrowDecoder:   events.NewEscrowDecoder(),
		transferDecoder: events.NewTransferDecoder(),
	}
}

func (p *EventProcessor) ProcessEvent(ctx context.Context, event stellar.SorobanEvent) error {
	eventType := stellar.DecodeTopicToEventName(event.Topics[0])
	
	log.Info().
		Str("type", eventType).
		Str("contract", event.ContractID).
		Uint32("ledger", event.Ledger).
		Msg("Processing event")
	
	switch eventType {
	case "tw_init":
		return p.processTwInit(ctx, event)
	case "tw_fund":
		return p.processTwFund(ctx, event)
	case "tw_ms_approve":
		return p.processTwMsApprove(ctx, event)
	case "tw_release":
		return p.processTwRelease(ctx, event)
	case "tw_dispute":
		return p.processTwDispute(ctx, event)
	case "transfer":
		return p.processTransfer(ctx, event)
	default:
		log.Warn().Str("type", eventType).Msg("Unknown event type, skipping")
		return nil
	}
}

func (p *EventProcessor) processTwInit(ctx context.Context, event stellar.SorobanEvent) error {
	initEvent, err := p.escrowDecoder.DecodeTwInit(event)
	if err != nil {
		return fmt.Errorf("failed to decode tw_init: %w", err)
	}
	
	tx, err := p.store.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	
	// 1. Create escrow
	if err := p.store.CreateEscrow(ctx, tx, initEvent); err != nil {
		return fmt.Errorf("failed to create escrow: %w", err)
	}
	
	// 2. Create milestones
	if err := p.store.CreateMilestones(ctx, tx, initEvent.ContractID, initEvent.Milestones, initEvent.OccurredAt); err != nil {
		return fmt.Errorf("failed to create milestones: %w", err)
	}
	
	// 3. Insert event
	if err := p.store.InsertEvent(ctx, tx, initEvent.ContractID, "tw_init", initEvent, initEvent.Ledger, initEvent.TxHash, initEvent.OccurredAt); err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}
	
	return tx.Commit(ctx)
}

func (p *EventProcessor) processTwMsApprove(ctx context.Context, event stellar.SorobanEvent) error {
	approveEvent, err := p.escrowDecoder.DecodeTwMsApprove(event)
	if err != nil {
		return fmt.Errorf("failed to decode tw_ms_approve: %w", err)
	}
	
	tx, err := p.store.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	
	// 1. Approve milestone
	if err := p.store.ApproveMilestone(ctx, tx, approveEvent.ContractID, approveEvent.MilestoneIndex, approveEvent.OccurredAt); err != nil {
		return fmt.Errorf("failed to approve milestone: %w", err)
	}
	
	// 2. Check if all approved
	allApproved, err := p.store.CheckAllMilestonesApproved(ctx, tx, approveEvent.ContractID)
	if err != nil {
		return err
	}
	
	if allApproved {
		if err := p.store.UpdateEscrowStatus(ctx, tx, approveEvent.ContractID, "approved", approveEvent.OccurredAt); err != nil {
			return err
		}
	}
	
	// 3. Insert event
	if err := p.store.InsertEvent(ctx, tx, approveEvent.ContractID, "tw_ms_approve", approveEvent, approveEvent.Ledger, approveEvent.TxHash, approveEvent.OccurredAt); err != nil {
		return err
	}
	
	return tx.Commit(ctx)
}

func (p *EventProcessor) processTwRelease(ctx context.Context, event stellar.SorobanEvent) error {
	releaseEvent, err := p.escrowDecoder.DecodeTwRelease(event)
	if err != nil {
		return fmt.Errorf("failed to decode tw_release: %w", err)
	}
	
	tx, err := p.store.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	
	if releaseEvent.MilestoneIndex != nil {
		// Multi-release: release milestone específico
		if err := p.store.ReleaseMilestone(ctx, tx, releaseEvent.ContractID, *releaseEvent.MilestoneIndex, releaseEvent.OccurredAt); err != nil {
			return err
		}
		
		// Check if all released
		allReleased, err := p.store.CheckAllMilestonesReleased(ctx, tx, releaseEvent.ContractID)
		if err != nil {
			return err
		}
		
		if allReleased {
			if err := p.store.UpdateEscrowStatus(ctx, tx, releaseEvent.ContractID, "completed", releaseEvent.OccurredAt); err != nil {
				return err
			}
		}
	} else {
		// Single-release: release todo
		if err := p.store.MarkEscrowReleased(ctx, tx, releaseEvent.ContractID, releaseEvent.OccurredAt); err != nil {
			return err
		}
	}
	
	// Insert event
	if err := p.store.InsertEvent(ctx, tx, releaseEvent.ContractID, "tw_release", releaseEvent, releaseEvent.Ledger, releaseEvent.TxHash, releaseEvent.OccurredAt); err != nil {
		return err
	}
	
	return tx.Commit(ctx)
}

func (p *EventProcessor) processTwFund(ctx context.Context, event stellar.SorobanEvent) error {
	fundEvent, err := p.escrowDecoder.DecodeTwFund(event)
	if err != nil {
		return fmt.Errorf("failed to decode tw_fund: %w", err)
	}
	
	tx, err := p.store.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	
	if err := p.store.UpdateEscrowStatus(ctx, tx, fundEvent.ContractID, "funded", fundEvent.OccurredAt); err != nil {
		return err
	}
	
	if err := p.store.InsertEvent(ctx, tx, fundEvent.ContractID, "tw_fund", fundEvent, fundEvent.Ledger, fundEvent.TxHash, fundEvent.OccurredAt); err != nil {
		return err
	}
	
	return tx.Commit(ctx)
}

func (p *EventProcessor) processTwDispute(ctx context.Context, event stellar.SorobanEvent) error {
	disputeEvent, err := p.escrowDecoder.DecodeTwDispute(event)
	if err != nil {
		return fmt.Errorf("failed to decode tw_dispute: %w", err)
	}
	
	tx, err := p.store.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	
	if err := p.store.MarkEscrowDisputed(ctx, tx, disputeEvent.ContractID, disputeEvent.OccurredAt); err != nil {
		return err
	}
	
	if err := p.store.InsertEvent(ctx, tx, disputeEvent.ContractID, "tw_dispute", disputeEvent, disputeEvent.Ledger, disputeEvent.TxHash, disputeEvent.OccurredAt); err != nil {
		return err
	}
	
	return tx.Commit(ctx)
}

func (p *EventProcessor) processTransfer(ctx context.Context, event stellar.SorobanEvent) error {
	// TODO: Implementar lógica de deposits
	log.Info().Msg("Transfer event detected (not implemented yet)")
	return nil
}
```

### internal/indexer/service.go

```go
package indexer

import (
	"context"
	"fmt"
	"time"
	
	"github.com/Trustless-Work/Indexer/internal/config"
	"github.com/Trustless-Work/Indexer/internal/stellar"
	"github.com/Trustless-Work/Indexer/internal/store"
	"github.com/rs/zerolog/log"
)

type Service struct {
	config    *config.Config
	clients   *stellar.Clients
	store     *store.Store
	processor *EventProcessor
}

func NewService(cfg *config.Config, clients *stellar.Clients, store *store.Store) *Service {
	return &Service{
		config:    cfg,
		clients:   clients,
		store:     store,
		processor: NewEventProcessor(store),
	}
}

func (s *Service) Start(ctx context.Context) error {
	log.Info().Msg("Starting indexer service")
	
	// Load checkpoint
	checkpoint, err := s.store.GetCheckpoint(ctx)
	if err != nil {
		return fmt.Errorf("failed to load checkpoint: %w", err)
	}
	
	log.Info().
		Uint32("ledger", checkpoint.LastLedgerSeq).
		Str("cursor", checkpoint.LastCursor).
		Msg("Loaded checkpoint")
	
	ticker := time.NewTicker(time.Duration(s.config.PollInterval) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Indexer stopping...")
			return ctx.Err()
			
		case <-ticker.C:
			if err := s.poll(ctx, checkpoint); err != nil {
				log.Error().Err(err).Msg("Poll failed")
			}
		}
	}
}

func (s *Service) poll(ctx context.Context, checkpoint *store.Checkpoint) error {
	// Build filters para todos los contratos de escrow
	filters := []stellar.EventFilter{}
	
	for _, contractID := range s.config.EscrowContractIDs {
		filters = append(filters,
			s.clients.BuildEscrowInitFilter(contractID),
			s.clients.BuildEscrowFundFilter(contractID),
			s.clients.BuildMilestoneApprovalFilter(contractID),
			s.clients.BuildEscrowReleaseFilter(contractID),
			s.clients.BuildEscrowDisputeFilter(contractID),
		)
	}
	
	// Construir request
	req := stellar.GetEventsRequest{
		Filters: filters,
		Pagination: &stellar.PaginationParam{
			Limit: s.config.BatchSize,
		},
	}
	
	if checkpoint.LastCursor != "" {
		req.Cursor = checkpoint.LastCursor
	} else {
		req.StartLedger = s.config.StartLedger
	}
	
	// Get events
	resp, err := s.clients.RPC.GetEvents(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}
	
	if len(resp.Events) == 0 {
		return nil
	}
	
	log.Info().
		Int("count", len(resp.Events)).
		Uint32("latest_ledger", resp.LatestLedger).
		Msg("Received events")
	
	// Process each event
	for _, event := range resp.Events {
		if err := s.processor.ProcessEvent(ctx, event); err != nil {
			log.Error().
				Err(err).
				Str("event_id", event.ID).
				Str("type", stellar.DecodeTopicToEventName(event.Topics[0])).
				Msg("Failed to process event")
			continue
		}
	}
	
	// Update checkpoint
	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	
	if err := s.store.UpdateCheckpoint(ctx, tx, resp.Cursor, resp.LatestLedger); err != nil {
		return err
	}
	
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	
	checkpoint.LastCursor = resp.Cursor
	checkpoint.LastLedgerSeq = resp.LatestLedger
	
	return nil
}
```

---

## Paso 7: API REST

### internal/api/server.go

```go
package api

import (
	"context"
	"fmt"
	"net/http"
	"time"
	
	"github.com/Trustless-Work/Indexer/internal/config"
	"github.com/Trustless-Work/Indexer/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

type Server struct {
	config *config.Config
	store  *store.Store
	router *chi.Mux
}

func NewServer(cfg *config.Config, store *store.Store) *Server {
	s := &Server{
		config: cfg,
		store:  store,
		router: chi.NewRouter(),
	}
	
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))
	
	s.router.Get("/health", s.handleHealth)
	s.router.Get("/escrow/{id}", s.handleGetEscrow)
	s.router.Get("/escrow/{id}/history", s.handleGetEscrowHistory)
	s.router.Get("/escrows", s.handleListEscrows)
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%s", s.config.APIPort)
	log.Info().Str("addr", addr).Msg("Starting API server")
	
	return http.ListenAndServe(addr, s.router)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleGetEscrow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	escrow, err := s.store.GetEscrow(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	respondJSON(w, escrow)
}

func (s *Server) handleGetEscrowHistory(w http.ResponseWriter, r *http.Request) {
	// TODO: Implementar
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) handleListEscrows(w http.ResponseWriter, r *http.Request) {
	// TODO: Implementar
	w.WriteHeader(http.StatusNotImplemented)
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
```

---

## Paso 8: Main Entry Point

### cmd/indexer/main.go

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	
	"github.com/Trustless-Work/Indexer/internal/api"
	"github.com/Trustless-Work/Indexer/internal/config"
	"github.com/Trustless-Work/Indexer/internal/indexer"
	"github.com/Trustless-Work/Indexer/internal/stellar"
	"github.com/Trustless-Work/Indexer/internal/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Setup logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	
	// Load config
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}
	
	// Setup store
	store, err := store.NewStore(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create store")
	}
	defer store.Close()
	
	// Setup Stellar clients
	clients, err := stellar.NewClients(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Stellar clients")
	}
	
	// Setup indexer service
	indexerService := indexer.NewService(cfg, clients, store)
	
	// Setup API server
	apiServer := api.NewServer(cfg, store)
	
	// Start services
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start indexer in goroutine
	go func() {
		if err := indexerService.Start(ctx); err != nil {
			log.Error().Err(err).Msg("Indexer stopped")
		}
	}()
	
	// Start API in goroutine
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Error().Err(err).Msg("API server stopped")
		}
	}()
	
	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	
	log.Info().Msg("Shutting down gracefully...")
	cancel()
}
```

---

## Anexo: Tipos de Datos

### pkg/types/escrow.go

```go
package types

import "time"

type EscrowInitEvent struct {
	ContractID    string
	EngagementID  string
	ContractType  string
	Title         string
	Description   string
	Amount        string
	PlatformFee   int32
	ReceiverMemo  string
	Roles         EscrowRoles
	Milestones    []MilestoneInit
	Trustline     TrustlineInfo
	Ledger        uint32
	TxHash        string
	OccurredAt    string
}

type EscrowRoles struct {
	Approver        string
	ServiceProvider string
	PlatformAddress string
	ReleaseSigner   string
	DisputeResolver string
	Receiver        string
}

type MilestoneInit struct {
	Description string
	Evidence    string
	Status      string
	Approved    *bool
	Amount      *string
	Receiver    *string
}

type TrustlineInfo struct {
	Address string
}

type MilestoneApproveEvent struct {
	ContractID     string
	MilestoneIndex uint32
	Ledger         uint32
	TxHash         string
	OccurredAt     string
}

type EscrowReleaseEvent struct {
	ContractID     string
	MilestoneIndex *uint32
	Ledger         uint32
	TxHash         string
	OccurredAt     string
}

type EscrowFundEvent struct {
	ContractID string
	Ledger     uint32
	TxHash     string
	OccurredAt string
}

type EscrowDisputeEvent struct {
	ContractID string
	Ledger     uint32
	TxHash     string
	OccurredAt string
}

type Escrow struct {
	ID               string
	EngagementID     string
	ContractType     string
	Status           string
	Title            string
	Description      string
	ReceiverMemo     string
	PlatformFee      int32
	Amount           *string
	Receiver         *string
	Approver         string
	ServiceProvider  string
	PlatformAddress  string
	ReleaseSigner    string
	DisputeResolver  string
	IsDisputed       bool
	IsReleased       bool
	IsResolved       bool
	TrustlineAddress string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
```

### pkg/types/deposit.go

```go
package types

type DepositEntry struct {
	ID              string
	EscrowID        *string
	FromAddress     string
	ToAddress       string
	Amount          string
	Asset           string
	LedgerSequence  int64
	TransactionHash string
	ContractID      string
	Processed       bool
	OccurredAt      string
	IndexedAt       string
}
```

---

## Comandos de Ejecución

```bash
# 1. Setup database
docker-compose up -d

# 2. Run migrations
psql -h localhost -U indexer -d escrow_indexer -f migrations/001_initial_schema.sql
psql -h localhost -U indexer -d escrow_indexer -f migrations/002_add_indexes.sql

# 3. Build
go build -o bin/indexer cmd/indexer/main.go

# 4. Run
./bin/indexer

# 5. Test API
curl http://localhost:8080/health
curl http://localhost:8080/escrow/CDQJ...XXXX
```

---

## Deployment en Railway

### Dockerfile

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o indexer cmd/indexer/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/indexer .
COPY --from=builder /app/configs ./configs

CMD ["./indexer"]
```

### railway.toml

```toml
[build]
builder = "dockerfile"

[deploy]
startCommand = "./indexer"
```

### Variables de entorno en Railway

```
DATABASE_URL=postgres://user:pass@host:5432/dbname
NETWORK=testnet
HORIZON_URL=https://horizon-testnet.stellar.org
SOROBAN_RPC_URL=https://soroban-testnet.stellar.org
API_PORT=8080
```

---

## Resumen de Flujo de Implementación

1. ✅ Setup proyecto Go + dependencias
2. ✅ Crear esquema PostgreSQL (migrations)
3. ✅ Implementar RPC client
4. ✅ Implementar decoders (escrow + transfer)
5. ✅ Implementar store layer
6. ✅ Implementar event processor
7. ✅ Implementar indexer service
8. ✅ Implementar API REST
9. ✅ Deploy a Railway

**Tiempo estimado:** 3-5 semanas para MVP completo.