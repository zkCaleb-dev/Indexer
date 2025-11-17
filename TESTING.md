# 🧪 Guía Completa de Pruebas - Stellar Indexer API

Esta guía proporciona todas las formas posibles de probar los endpoints de la API del indexer.

---

## 📋 Tabla de Contenidos

1. [Prerequisitos](#prerequisitos)
2. [Método 1: cURL (Línea de Comandos)](#método-1-curl-línea-de-comandos)
3. [Método 2: Postman](#método-2-postman)
4. [Método 3: Scripts en Go](#método-3-scripts-en-go)
5. [Método 4: Scripts en Python](#método-4-scripts-en-python)
6. [Método 5: HTTPie](#método-5-httpie)
7. [Método 6: Navegador Web](#método-6-navegador-web)
8. [Casos de Prueba Específicos](#casos-de-prueba-específicos)
9. [Validación de Respuestas](#validación-de-respuestas)
10. [Troubleshooting](#troubleshooting)

---

## Prerequisitos

### 1. Iniciar el Indexer

```bash
# Asegúrate de que el indexer esté corriendo
./indexer

# Deberías ver:
# API server started successfully port=2112
```

### 2. Verificar que la API está activa

```bash
curl http://localhost:2112/health
```

**Respuesta esperada:**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:30:00Z",
  "service": "stellar-indexer"
}
```

### 3. Obtener un Contract ID de prueba

```bash
# Listar contratos disponibles
curl http://localhost:2112/contracts | jq '.contracts[0].contract_id'

# Guarda este ID para las pruebas
export CONTRACT_ID="CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
```

---

## Método 1: cURL (Línea de Comandos)

### Instalación

```bash
# Linux/Mac (usualmente viene preinstalado)
which curl

# Si no está instalado:
# Ubuntu/Debian
sudo apt-get install curl

# Mac
brew install curl
```

### Pruebas Básicas

#### 1. Health Check
```bash
curl http://localhost:2112/health
```

#### 2. Service Info
```bash
curl http://localhost:2112/
```

#### 3. Prometheus Metrics
```bash
curl http://localhost:2112/metrics
```

### Pruebas de Contratos

#### 1. Listar Todos los Contratos

```bash
# Sin formato
curl http://localhost:2112/contracts

# Con formato JSON (requiere jq)
curl http://localhost:2112/contracts | jq .

# Con pretty print
curl -s http://localhost:2112/contracts | jq .
```

#### 2. Listar con Filtros

```bash
# Filtrar por tipo
curl "http://localhost:2112/contracts?type=single-release" | jq .
curl "http://localhost:2112/contracts?type=multi-release" | jq .

# Con paginación
curl "http://localhost:2112/contracts?limit=10&offset=0" | jq .
curl "http://localhost:2112/contracts?limit=10&offset=10" | jq .

# Filtrar por deployer
curl "http://localhost:2112/contracts?deployer=GAXXX..." | jq .

# Múltiples filtros
curl "http://localhost:2112/contracts?type=multi-release&limit=5" | jq .
```

#### 3. Obtener Detalle de Contrato

```bash
# Reemplaza CONTRACT_ID con un ID real
CONTRACT_ID="CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"

# Detalle completo
curl "http://localhost:2112/contracts/${CONTRACT_ID}" | jq .

# Extraer solo información específica
curl "http://localhost:2112/contracts/${CONTRACT_ID}" | jq '.title'
curl "http://localhost:2112/contracts/${CONTRACT_ID}" | jq '.status'
curl "http://localhost:2112/contracts/${CONTRACT_ID}" | jq '.milestones'
curl "http://localhost:2112/contracts/${CONTRACT_ID}" | jq '.roles'
```

#### 4. Obtener Eventos de Contrato

```bash
# Todos los eventos
curl "http://localhost:2112/contracts/${CONTRACT_ID}/events" | jq .

# Contar eventos
curl "http://localhost:2112/contracts/${CONTRACT_ID}/events" | jq '.total'

# Listar tipos de eventos
curl "http://localhost:2112/contracts/${CONTRACT_ID}/events" | jq '.events[].event_type'

# Filtrar eventos específicos (usando jq)
curl "http://localhost:2112/contracts/${CONTRACT_ID}/events" | \
  jq '.events[] | select(.event_type == "tw_ms_approve")'
```

#### 5. Obtener Milestones de Contrato

```bash
# Todos los milestones
curl "http://localhost:2112/contracts/${CONTRACT_ID}/milestones" | jq .

# Solo milestones aprobados
curl "http://localhost:2112/contracts/${CONTRACT_ID}/milestones" | \
  jq '.milestones[] | select(.approved == true)'

# Solo milestones liberados
curl "http://localhost:2112/contracts/${CONTRACT_ID}/milestones" | \
  jq '.milestones[] | select(.released == true)'

# Milestone específico por índice
curl "http://localhost:2112/contracts/${CONTRACT_ID}/milestones" | \
  jq '.milestones[0]'
```

### Guardar Respuestas en Archivos

```bash
# Guardar JSON completo
curl "http://localhost:2112/contracts/${CONTRACT_ID}" > contract_detail.json

# Guardar con timestamp
curl "http://localhost:2112/contracts/${CONTRACT_ID}" > "contract_$(date +%Y%m%d_%H%M%S).json"

# Guardar múltiples endpoints
curl "http://localhost:2112/contracts" > contracts_list.json
curl "http://localhost:2112/contracts/${CONTRACT_ID}" > contract_detail.json
curl "http://localhost:2112/contracts/${CONTRACT_ID}/events" > contract_events.json
curl "http://localhost:2112/contracts/${CONTRACT_ID}/milestones" > contract_milestones.json
```

### Medición de Performance

```bash
# Medir tiempo de respuesta
curl -w "\nTime Total: %{time_total}s\n" \
  -o /dev/null -s \
  "http://localhost:2112/contracts"

# Medir con headers
curl -w "\nHTTP Code: %{http_code}\nTime: %{time_total}s\n" \
  -o /dev/null -s \
  "http://localhost:2112/contracts/${CONTRACT_ID}"

# Benchmark completo
curl -w @- -o /dev/null -s "http://localhost:2112/contracts/${CONTRACT_ID}" <<'EOF'
    time_namelookup:  %{time_namelookup}\n
       time_connect:  %{time_connect}\n
    time_appconnect:  %{time_appconnect}\n
   time_pretransfer:  %{time_pretransfer}\n
      time_redirect:  %{time_redirect}\n
 time_starttransfer:  %{time_starttransfer}\n
                    ----------\n
         time_total:  %{time_total}\n
           http_code:  %{http_code}\n
EOF
```

---

## Método 2: Postman

### Instalación

1. Descargar desde: https://www.postman.com/downloads/
2. O usar la versión web: https://web.postman.com/

### Configuración

#### 1. Crear Nueva Colección

1. Abrir Postman
2. Click en "Collections" → "+"
3. Nombre: "Stellar Indexer API"

#### 2. Configurar Variables de Entorno

1. Click en "Environments" → "+"
2. Nombre: "Local Development"
3. Agregar variables:

| Variable | Initial Value | Current Value |
|----------|---------------|---------------|
| base_url | http://localhost:2112 | http://localhost:2112 |
| contract_id | CDLZFC3SYJYDZT7K67VZ75... | CDLZFC3SYJYDZT7K67VZ75... |

#### 3. Crear Requests

**Request 1: Health Check**
- Method: `GET`
- URL: `{{base_url}}/health`
- Tests:
```javascript
pm.test("Status is 200", function () {
    pm.response.to.have.status(200);
});

pm.test("Service is healthy", function () {
    const jsonData = pm.response.json();
    pm.expect(jsonData.status).to.eql("healthy");
});
```

**Request 2: List Contracts**
- Method: `GET`
- URL: `{{base_url}}/contracts`
- Params:
  - `limit`: `50`
  - `offset`: `0`
  - `type`: `multi-release` (opcional)
- Tests:
```javascript
pm.test("Status is 200", function () {
    pm.response.to.have.status(200);
});

pm.test("Has contracts array", function () {
    const jsonData = pm.response.json();
    pm.expect(jsonData).to.have.property("contracts");
    pm.expect(jsonData.contracts).to.be.an("array");
});

// Guardar primer contract_id para otros requests
if (pm.response.json().contracts.length > 0) {
    pm.environment.set("contract_id", pm.response.json().contracts[0].contract_id);
}
```

**Request 3: Get Contract Detail**
- Method: `GET`
- URL: `{{base_url}}/contracts/{{contract_id}}`
- Tests:
```javascript
pm.test("Status is 200", function () {
    pm.response.to.have.status(200);
});

pm.test("Has contract details", function () {
    const jsonData = pm.response.json();
    pm.expect(jsonData).to.have.property("contract_id");
    pm.expect(jsonData).to.have.property("status");
    pm.expect(jsonData).to.have.property("milestones");
});

pm.test("Amount is in XLM format", function () {
    const jsonData = pm.response.json();
    if (jsonData.amount_xlm) {
        pm.expect(jsonData.amount_xlm).to.match(/^\d+\.\d{7}$/);
    }
});
```

**Request 4: Get Contract Events**
- Method: `GET`
- URL: `{{base_url}}/contracts/{{contract_id}}/events`

**Request 5: Get Contract Milestones**
- Method: `GET`
- URL: `{{base_url}}/contracts/{{contract_id}}/milestones`

### Exportar Colección

```bash
# Guardar este JSON como "Stellar_Indexer.postman_collection.json"
```

<details>
<summary>Ver JSON completo de Colección Postman</summary>

```json
{
  "info": {
    "name": "Stellar Indexer API",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "item": [
    {
      "name": "Health Check",
      "request": {
        "method": "GET",
        "header": [],
        "url": {
          "raw": "{{base_url}}/health",
          "host": ["{{base_url}}"],
          "path": ["health"]
        }
      }
    },
    {
      "name": "List Contracts",
      "request": {
        "method": "GET",
        "header": [],
        "url": {
          "raw": "{{base_url}}/contracts?limit=50",
          "host": ["{{base_url}}"],
          "path": ["contracts"],
          "query": [
            {"key": "limit", "value": "50"},
            {"key": "offset", "value": "0", "disabled": true},
            {"key": "type", "value": "multi-release", "disabled": true}
          ]
        }
      }
    },
    {
      "name": "Get Contract",
      "request": {
        "method": "GET",
        "header": [],
        "url": {
          "raw": "{{base_url}}/contracts/{{contract_id}}",
          "host": ["{{base_url}}"],
          "path": ["contracts", "{{contract_id}}"]
        }
      }
    },
    {
      "name": "Get Contract Events",
      "request": {
        "method": "GET",
        "header": [],
        "url": {
          "raw": "{{base_url}}/contracts/{{contract_id}}/events",
          "host": ["{{base_url}}"],
          "path": ["contracts", "{{contract_id}}", "events"]
        }
      }
    },
    {
      "name": "Get Contract Milestones",
      "request": {
        "method": "GET",
        "header": [],
        "url": {
          "raw": "{{base_url}}/contracts/{{contract_id}}/milestones",
          "host": ["{{base_url}}"],
          "path": ["contracts", "{{contract_id}}", "milestones"]
        }
      }
    }
  ]
}
```
</details>

---

## Método 3: Scripts en Go

### Script de Prueba Completo

```go
// test_api.go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "http://localhost:2112"

type ContractListResponse struct {
	Contracts []ContractSummary `json:"contracts"`
	Total     int               `json:"total"`
	Page      int               `json:"page"`
	PageSize  int               `json:"page_size"`
}

type ContractSummary struct {
	ContractID   string    `json:"contract_id"`
	EngagementID string    `json:"engagement_id"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	AmountXLM    string    `json:"amount_xlm"`
	Status       string    `json:"status"`
	DeployedAt   time.Time `json:"deployed_at"`
}

func main() {
	fmt.Println("🧪 Testing Stellar Indexer API")
	fmt.Println("================================")

	// Test 1: Health Check
	fmt.Println("\n1. Testing Health Check...")
	if err := testHealthCheck(); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Println("✅ Passed")
	}

	// Test 2: List Contracts
	fmt.Println("\n2. Testing List Contracts...")
	contractID, err := testListContracts()
	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		return
	}
	fmt.Println("✅ Passed")

	if contractID == "" {
		fmt.Println("⚠️  No contracts found, skipping remaining tests")
		return
	}

	// Test 3: Get Contract Detail
	fmt.Println("\n3. Testing Get Contract Detail...")
	if err := testGetContract(contractID); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Println("✅ Passed")
	}

	// Test 4: Get Contract Events
	fmt.Println("\n4. Testing Get Contract Events...")
	if err := testGetEvents(contractID); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Println("✅ Passed")
	}

	// Test 5: Get Contract Milestones
	fmt.Println("\n5. Testing Get Contract Milestones...")
	if err := testGetMilestones(contractID); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Println("✅ Passed")
	}

	fmt.Println("\n================================")
	fmt.Println("✅ All tests completed!")
}

func testHealthCheck() error {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result["status"] != "healthy" {
		return fmt.Errorf("service not healthy")
	}

	return nil
}

func testListContracts() (string, error) {
	resp, err := http.Get(baseURL + "/contracts?limit=10")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result ContractListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	fmt.Printf("   Found %d contracts (page %d)\n", len(result.Contracts), result.Page)

	if len(result.Contracts) == 0 {
		return "", nil
	}

	contractID := result.Contracts[0].ContractID
	fmt.Printf("   Using contract: %s\n", contractID[:16]+"...")

	return contractID, nil
}

func testGetContract(contractID string) error {
	url := fmt.Sprintf("%s/contracts/%s", baseURL, contractID)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	fmt.Printf("   Title: %s\n", result["title"])
	fmt.Printf("   Status: %s\n", result["status"])
	if milestones, ok := result["milestones"].([]interface{}); ok {
		fmt.Printf("   Milestones: %d\n", len(milestones))
	}

	return nil
}

func testGetEvents(contractID string) error {
	url := fmt.Sprintf("%s/contracts/%s/events", baseURL, contractID)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	fmt.Printf("   Total events: %v\n", result["total"])

	return nil
}

func testGetMilestones(contractID string) error {
	url := fmt.Sprintf("%s/contracts/%s/milestones", baseURL, contractID)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	fmt.Printf("   Total milestones: %v\n", result["total"])

	return nil
}
```

**Ejecutar:**
```bash
go run test_api.go
```

---

## Método 4: Scripts en Python

### Script de Prueba Completo

```python
#!/usr/bin/env python3
# test_api.py

import requests
import json
from datetime import datetime

BASE_URL = "http://localhost:2112"

def test_health_check():
    """Test health check endpoint"""
    print("\n1. Testing Health Check...")
    response = requests.get(f"{BASE_URL}/health")

    assert response.status_code == 200, f"Expected 200, got {response.status_code}"
    data = response.json()
    assert data["status"] == "healthy", "Service not healthy"

    print("✅ Passed")
    return True

def test_list_contracts():
    """Test list contracts endpoint"""
    print("\n2. Testing List Contracts...")
    response = requests.get(f"{BASE_URL}/contracts", params={"limit": 10})

    assert response.status_code == 200, f"Expected 200, got {response.status_code}"
    data = response.json()

    print(f"   Found {len(data['contracts'])} contracts (page {data['page']})")

    if len(data['contracts']) == 0:
        print("⚠️  No contracts found")
        return None

    contract_id = data['contracts'][0]['contract_id']
    print(f"   Using contract: {contract_id[:16]}...")
    print("✅ Passed")

    return contract_id

def test_get_contract(contract_id):
    """Test get contract detail endpoint"""
    print("\n3. Testing Get Contract Detail...")
    response = requests.get(f"{BASE_URL}/contracts/{contract_id}")

    assert response.status_code == 200, f"Expected 200, got {response.status_code}"
    data = response.json()

    assert "contract_id" in data
    assert "status" in data
    assert "milestones" in data

    print(f"   Title: {data.get('title', 'N/A')}")
    print(f"   Status: {data['status']}")
    print(f"   Milestones: {len(data['milestones'])}")

    print("✅ Passed")
    return data

def test_get_events(contract_id):
    """Test get contract events endpoint"""
    print("\n4. Testing Get Contract Events...")
    response = requests.get(f"{BASE_URL}/contracts/{contract_id}/events")

    assert response.status_code == 200, f"Expected 200, got {response.status_code}"
    data = response.json()

    print(f"   Total events: {data['total']}")

    if data['total'] > 0:
        print(f"   Event types: {', '.join(set(e['event_type'] for e in data['events']))}")

    print("✅ Passed")
    return data

def test_get_milestones(contract_id):
    """Test get contract milestones endpoint"""
    print("\n5. Testing Get Contract Milestones...")
    response = requests.get(f"{BASE_URL}/contracts/{contract_id}/milestones")

    assert response.status_code == 200, f"Expected 200, got {response.status_code}"
    data = response.json()

    print(f"   Total milestones: {data['total']}")

    if data['total'] > 0:
        for ms in data['milestones']:
            print(f"   - [{ms['index']}] {ms['status']}: {ms['description'][:50]}")

    print("✅ Passed")
    return data

def test_filters():
    """Test query parameter filters"""
    print("\n6. Testing Query Filters...")

    # Test type filter
    response = requests.get(f"{BASE_URL}/contracts", params={"type": "multi-release"})
    assert response.status_code == 200
    data = response.json()
    print(f"   Multi-release contracts: {len(data['contracts'])}")

    # Test pagination
    response = requests.get(f"{BASE_URL}/contracts", params={"limit": 5, "offset": 0})
    assert response.status_code == 200
    data = response.json()
    assert data['page_size'] == 5
    print(f"   Pagination works (page_size: {data['page_size']})")

    print("✅ Passed")

def save_responses(contract_id):
    """Save all responses to JSON files"""
    print("\n7. Saving Responses to Files...")

    endpoints = {
        "contracts_list": f"{BASE_URL}/contracts",
        "contract_detail": f"{BASE_URL}/contracts/{contract_id}",
        "contract_events": f"{BASE_URL}/contracts/{contract_id}/events",
        "contract_milestones": f"{BASE_URL}/contracts/{contract_id}/milestones",
    }

    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")

    for name, url in endpoints.items():
        response = requests.get(url)
        filename = f"test_output/{name}_{timestamp}.json"

        with open(filename, 'w') as f:
            json.dump(response.json(), f, indent=2)

        print(f"   Saved: {filename}")

    print("✅ Passed")

def main():
    print("🧪 Testing Stellar Indexer API")
    print("================================")

    try:
        # Run all tests
        test_health_check()
        contract_id = test_list_contracts()

        if contract_id is None:
            print("\n⚠️  No contracts found, skipping remaining tests")
            return

        test_get_contract(contract_id)
        test_get_events(contract_id)
        test_get_milestones(contract_id)
        test_filters()

        # Save responses (optional)
        import os
        os.makedirs("test_output", exist_ok=True)
        save_responses(contract_id)

        print("\n================================")
        print("✅ All tests completed!")

    except AssertionError as e:
        print(f"\n❌ Test failed: {e}")
    except Exception as e:
        print(f"\n❌ Error: {e}")

if __name__ == "__main__":
    main()
```

**Ejecutar:**
```bash
# Instalar dependencias
pip install requests

# Ejecutar tests
python test_api.py
```

---

## Método 5: HTTPie

HTTPie es una alternativa más user-friendly a cURL.

### Instalación

```bash
# Ubuntu/Debian
sudo apt install httpie

# Mac
brew install httpie

# Python
pip install httpie
```

### Ejemplos de Uso

```bash
# Health check
http GET localhost:2112/health

# List contracts (auto pretty-print)
http GET localhost:2112/contracts

# With query parameters
http GET localhost:2112/contracts type==multi-release limit==10

# Get contract detail
http GET localhost:2112/contracts/CDLZFC3SYJYDZT7K67VZ75...

# Download response
http GET localhost:2112/contracts > contracts.json

# Show headers
http -v GET localhost:2112/contracts

# Benchmark
time http GET localhost:2112/contracts
```

---

## Método 6: Navegador Web

### Endpoints Accesibles por Navegador

Simplemente abre tu navegador y navega a:

```
http://localhost:2112/
http://localhost:2112/health
http://localhost:2112/metrics
http://localhost:2112/contracts
http://localhost:2112/contracts/CDLZFC3SYJYDZT7K67VZ75...
http://localhost:2112/contracts/CDLZFC3SYJYDZT7K67VZ75.../events
http://localhost:2112/contracts/CDLZFC3SYJYDZT7K67VZ75.../milestones
```

### Extensiones Útiles

**Chrome:**
- JSON Formatter: https://chrome.google.com/webstore/detail/json-formatter
- REST Client: https://chrome.google.com/webstore/detail/talend-api-tester

**Firefox:**
- JSONView: https://addons.mozilla.org/en-US/firefox/addon/jsonview/

---

## Casos de Prueba Específicos

### Caso 1: Contrato Nuevo (Sin Funding)

```bash
# Buscar contratos pending_funding
curl "http://localhost:2112/contracts" | \
  jq '.contracts[] | select(.status == "pending_funding")'
```

### Caso 2: Contrato con Dispute

```bash
# Buscar contratos disputed
curl "http://localhost:2112/contracts" | \
  jq '.contracts[] | select(.status == "disputed")'

# Ver detalles de la disputa
CONTRACT_ID="xxx"
curl "http://localhost:2112/contracts/${CONTRACT_ID}/events" | \
  jq '.events[] | select(.event_type == "tw_dispute")'
```

### Caso 3: Contrato Completado

```bash
# Buscar contratos completados
curl "http://localhost:2112/contracts" | \
  jq '.contracts[] | select(.status == "completed")'

# Verificar todos los milestones liberados
curl "http://localhost:2112/contracts/${CONTRACT_ID}/milestones" | \
  jq '.milestones | map(.released) | all'
```

### Caso 4: Timeline de un Milestone

```bash
# Ver toda la historia de un milestone específico
curl "http://localhost:2112/contracts/${CONTRACT_ID}/events" | \
  jq '.events[] | select(.data.parsed.milestone_index == 0)'
```

### Caso 5: Verificar Conversión XLM

```bash
# Verificar que amounts estén en formato correcto
curl "http://localhost:2112/contracts/${CONTRACT_ID}" | \
  jq '{
    amount_stroops: .amount_stroops,
    amount_xlm: .amount_xlm,
    balance_stroops: .balance_stroops,
    balance_xlm: .balance_xlm
  }'
```

### Caso 6: Comparar Single vs Multi Release

```bash
# Single-release
curl "http://localhost:2112/contracts?type=single-release&limit=1" | \
  jq '.contracts[0] | {type, amount_xlm, roles}'

# Multi-release
curl "http://localhost:2112/contracts?type=multi-release&limit=1" | \
  jq '.contracts[0] | {type, milestones: .milestones | length}'
```

---

## Validación de Respuestas

### Validar Estructura JSON

```bash
# Usando jq para validar esquema
curl "http://localhost:2112/contracts/${CONTRACT_ID}" | \
  jq 'if .contract_id and .status and .milestones then "✅ Valid" else "❌ Invalid" end'
```

### Validar Datos

```python
# validate_response.py
import requests
import json

def validate_contract_response(data):
    """Validate contract response structure"""
    required_fields = [
        'contract_id', 'type', 'status', 'roles',
        'milestones', 'deployed_at'
    ]

    for field in required_fields:
        assert field in data, f"Missing required field: {field}"

    # Validate contract_id format (starts with C, 56 chars)
    assert data['contract_id'].startswith('C'), "Invalid contract_id prefix"
    assert len(data['contract_id']) == 56, "Invalid contract_id length"

    # Validate status
    valid_statuses = ['pending_funding', 'active', 'disputed', 'completed']
    assert data['status'] in valid_statuses, f"Invalid status: {data['status']}"

    # Validate XLM format (7 decimals)
    if 'amount_xlm' in data and data['amount_xlm']:
        parts = data['amount_xlm'].split('.')
        assert len(parts) == 2, "Invalid XLM format"
        assert len(parts[1]) == 7, "XLM should have 7 decimals"

    # Validate milestones
    if data['milestones']:
        for i, ms in enumerate(data['milestones']):
            assert ms['index'] == i, f"Milestone index mismatch: {i}"
            assert 'status' in ms, f"Milestone {i} missing status"

    print("✅ Response structure is valid")
    return True

# Test
response = requests.get(f"http://localhost:2112/contracts/CXXX...")
validate_contract_response(response.json())
```

---

## Troubleshooting

### Problema: "Connection refused"

```bash
# Verificar que el indexer esté corriendo
ps aux | grep indexer

# Verificar el puerto
lsof -i :2112

# Reiniciar el indexer
./indexer
```

### Problema: "404 Not Found"

```bash
# Verificar endpoint exacto
curl -v http://localhost:2112/contracts/WRONG_ID

# Lista de endpoints válidos
curl http://localhost:2112/ | jq '.endpoints'
```

### Problema: "Empty response"

```bash
# Verificar que haya datos en la DB
psql -U indexer -d stellar_indexer -c "SELECT COUNT(*) FROM deployed_contracts;"

# Verificar logs del indexer
tail -f indexer.log
```

### Problema: "Invalid JSON"

```bash
# Verificar formato JSON
curl http://localhost:2112/contracts | jq empty

# Si falla, ver raw response
curl http://localhost:2112/contracts
```

---

## Scripts Útiles de Bash

### Script de Prueba Rápida

```bash
#!/bin/bash
# quick_test.sh

BASE_URL="http://localhost:2112"

echo "🧪 Quick API Test"
echo "================="

# 1. Health
echo -n "Health check... "
if curl -sf "${BASE_URL}/health" > /dev/null; then
    echo "✅"
else
    echo "❌ FAILED"
    exit 1
fi

# 2. List
echo -n "List contracts... "
RESPONSE=$(curl -sf "${BASE_URL}/contracts")
COUNT=$(echo "$RESPONSE" | jq -r '.total')
echo "✅ ($COUNT contracts)"

# 3. Detail
if [ "$COUNT" -gt 0 ]; then
    CONTRACT_ID=$(echo "$RESPONSE" | jq -r '.contracts[0].contract_id')
    echo -n "Get contract detail... "
    if curl -sf "${BASE_URL}/contracts/${CONTRACT_ID}" > /dev/null; then
        echo "✅"
    else
        echo "❌ FAILED"
    fi
fi

echo "================="
echo "✅ All tests passed!"
```

**Ejecutar:**
```bash
chmod +x quick_test.sh
./quick_test.sh
```

---

## Resumen de Comandos Más Útiles

```bash
# Quick health check
curl -sf http://localhost:2112/health | jq .

# Get all contracts
curl http://localhost:2112/contracts | jq .

# Get first contract ID
CONTRACT_ID=$(curl -s http://localhost:2112/contracts | jq -r '.contracts[0].contract_id')

# Get contract detail
curl "http://localhost:2112/contracts/${CONTRACT_ID}" | jq .

# Get events timeline
curl "http://localhost:2112/contracts/${CONTRACT_ID}/events" | jq '.events[] | {type: .event_type, time: .timestamp}'

# Get milestone status
curl "http://localhost:2112/contracts/${CONTRACT_ID}/milestones" | jq '.milestones[] | {index, status, approved, released}'

# Save all data for a contract
mkdir -p contract_data
curl "http://localhost:2112/contracts/${CONTRACT_ID}" > "contract_data/detail.json"
curl "http://localhost:2112/contracts/${CONTRACT_ID}/events" > "contract_data/events.json"
curl "http://localhost:2112/contracts/${CONTRACT_ID}/milestones" > "contract_data/milestones.json"
```

---

## 📚 Recursos Adicionales

- **jq Tutorial**: https://stedolan.github.io/jq/tutorial/
- **Postman Learning**: https://learning.postman.com/
- **HTTPie Docs**: https://httpie.io/docs
- **cURL Cookbook**: https://catonmat.net/cookbooks/curl

---

**¡Happy Testing!** 🚀
