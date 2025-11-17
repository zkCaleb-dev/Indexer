# Retry Module

Módulo modular y configurable para manejo de reintentos con backoff exponencial.

## Características

- **Completamente modular**: Se puede activar/desactivar fácilmente
- **Estrategias intercambiables**: Usa el patrón Strategy para diferentes implementaciones
- **Backoff exponencial**: Previene saturar el servidor durante fallos
- **Detección de errores recuperables**: Distingue entre errores temporales y fatales
- **Altamente configurable**: Todos los parámetros configurables vía variables de entorno

## Arquitectura

```
retry/
├── strategy.go     # Interface Strategy y Factory
├── config.go       # Configuración del módulo
├── no_retry.go     # Estrategia sin reintentos (default)
└── exponential.go  # Estrategia con backoff exponencial
```

### Interface Strategy

```go
type Strategy interface {
    Execute(ctx context.Context, operation Operation) error
    Name() string
}
```

## Uso

### 1. Cargar configuración

```go
import "indexer/internal/ledger/retry"

// Carga configuración desde variables de entorno
config := retry.LoadConfig()
```

### 2. Crear estrategia

```go
// El factory crea la estrategia apropiada según la configuración
strategy := retry.NewStrategy(config)
```

### 3. Usar en el código

```go
// Ejecutar operación con retry automático
err := strategy.Execute(ctx, func() error {
    return someOperation()
})
```

## Configuración

### Variables de entorno

| Variable | Descripción | Default | Ejemplo |
|----------|-------------|---------|---------|
| `RETRY_ENABLED` | Habilitar/deshabilitar retry | `true` | `true` o `false` |
| `RETRY_MAX_RETRIES` | Máximo número de reintentos | `10` | `10` |
| `RETRY_INITIAL_DELAY_SEC` | Delay inicial en segundos | `1` | `1` |
| `RETRY_MAX_DELAY_SEC` | Delay máximo en segundos | `60` | `60` |

### Ejemplo de configuración en `.env`

```bash
# Activar retry
RETRY_ENABLED=true

# Configurar reintentos
RETRY_MAX_RETRIES=10
RETRY_INITIAL_DELAY_SEC=1
RETRY_MAX_DELAY_SEC=60
```

### Desactivar retry

Para volver al comportamiento anterior (sin reintentos):

```bash
RETRY_ENABLED=false
```

## Estrategias disponibles

### NoRetryStrategy

Ejecuta la operación una sola vez sin reintentos. Esta es la estrategia usada cuando `RETRY_ENABLED=false`.

```go
strategy := retry.NewNoRetryStrategy()
```

### ExponentialBackoffStrategy

Reintenta con backoff exponencial. Secuencia de delays:
- Intento 1: inmediato
- Intento 2: 1 segundo
- Intento 3: 2 segundos
- Intento 4: 4 segundos
- Intento 5: 8 segundos
- Intento 6: 16 segundos
- Intento 7: 32 segundos
- Intento 8+: 60 segundos (máximo)

```go
strategy := retry.NewExponentialBackoffStrategy(
    10,                    // max retries
    1 * time.Second,       // initial delay
    60 * time.Second,      // max delay
)
```

## Detección de errores recuperables

El módulo identifica automáticamente errores de red temporales:

- `connection reset by peer`
- `connection refused`
- `timeout`
- `network is unreachable`
- `broken pipe`
- `i/o timeout`
- `EOF`
- Y más...

Errores no recuperables fallan inmediatamente sin reintentar.

## Logging

El módulo genera logs detallados:

```
INFO  Retry enabled, using ExponentialBackoffStrategy max_retries=10 initial_delay_sec=1 max_delay_sec=60
WARN  Operation failed, retrying with exponential backoff attempt=2 max_attempts=11 retry_in_seconds=2 error="connection reset"
INFO  Operation succeeded after retry attempt=3 total_attempts=11
```

## Agregar nuevas estrategias

Para agregar una nueva estrategia:

1. Crear archivo `nueva_estrategia.go`
2. Implementar la interface `Strategy`:
   ```go
   type MiEstrategia struct {}

   func (s *MiEstrategia) Execute(ctx context.Context, op Operation) error {
       // Tu lógica aquí
   }

   func (s *MiEstrategia) Name() string {
       return "MiEstrategia"
   }
   ```
3. Actualizar el factory en `strategy.go` si es necesario

## Testing

Para probar el módulo:

```go
// Mockear una operación que falla
failCount := 0
err := strategy.Execute(context.Background(), func() error {
    failCount++
    if failCount < 3 {
        return errors.New("connection reset by peer")
    }
    return nil
})

// Debería tener éxito después de 2 reintentos
```

## Métricas

El módulo registra:
- Número de reintentos por operación
- Tiempo total de la operación incluyendo reintentos
- Errores recuperables vs no recuperables
