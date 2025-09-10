# Configuration Reference

This document provides a comprehensive reference for all configuration options available in the dYdX Order Routing Service.

## Configuration Sources

The service reads configuration from multiple sources in the following order of precedence:

1. **Command Line Arguments** (highest priority)
2. **Environment Variables**
3. **Configuration Files** (`.env`, `.env.local`)
4. **Default Values** (lowest priority)

## Environment Variables

### Server Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `PORT` | string | `8080` | HTTP server port |
| `HOST` | string | `0.0.0.0` | HTTP server bind address |
| `LOG_LEVEL` | string | `info` | Logging level (`debug`, `info`, `warn`, `error`) |
| `LOG_FORMAT` | string | `text` | Log format (`text`, `json`) |
| `LOG_OUTPUT` | string | `stdout` | Log output destination |
| `SHUTDOWN_TIMEOUT` | duration | `30s` | Graceful shutdown timeout |
| `READ_TIMEOUT` | duration | `30s` | HTTP read timeout |
| `WRITE_TIMEOUT` | duration | `30s` | HTTP write timeout |
| `IDLE_TIMEOUT` | duration | `120s` | HTTP idle timeout |

### Database Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DATABASE_URL` | string | **required** | PostgreSQL connection string |
| `DATABASE_HOST` | string | `localhost` | Database host |
| `DATABASE_PORT` | int | `5432` | Database port |
| `DATABASE_NAME` | string | `cryptopulse` | Database name |
| `DATABASE_USER` | string | `cryptopulse` | Database username |
| `DATABASE_PASSWORD` | string | **required** | Database password |
| `DATABASE_SSL_MODE` | string | `disable` | SSL mode (`disable`, `require`, `verify-ca`, `verify-full`) |
| `DATABASE_MAX_OPEN_CONNS` | int | `25` | Maximum open connections |
| `DATABASE_MAX_IDLE_CONNS` | int | `5` | Maximum idle connections |
| `DATABASE_CONN_MAX_LIFETIME` | duration | `5m` | Connection maximum lifetime |
| `DATABASE_QUERY_TIMEOUT` | duration | `30s` | Query timeout |
| `DATABASE_MIGRATION_PATH` | string | `file://pkg/database/migrations` | Migration files path |

### dYdX Network Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DYDX_NETWORK` | string | `testnet` | Network environment (`testnet`, `mainnet`) |
| `INDEXER_URL` | string | `https://indexer.v4testnet.dydx.exchange` | dYdX Indexer API URL |
| `RPC_URL` | string | `https://test-dydx-rpc.kingnodes.com:443` | Cosmos RPC endpoint |
| `CHAIN_ID` | string | `dydx-testnet-4` | Blockchain chain ID |
| `GAS_PRICE` | string | `0.025` | Gas price for transactions |
| `GAS_ADJUSTMENT` | float | `1.5` | Gas adjustment multiplier |
| `TRANSACTION_TIMEOUT` | duration | `60s` | Transaction broadcast timeout |

### Wallet Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `MNEMONIC` | string | **required** | BIP39 mnemonic phrase |
| `WALLET_HD_PATH` | string | `m/44'/118'/0'/0/0` | HD wallet derivation path |
| `WALLET_PREFIX` | string | `dydx` | Address prefix |

### Security Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `API_KEY` | string | **required** | API key for authentication |
| `RATE_LIMIT_REQUESTS_PER_MINUTE` | int | `100` | Rate limit per minute |
| `RATE_LIMIT_BURST` | int | `20` | Rate limit burst capacity |
| `RATE_LIMIT_CLEANUP_INTERVAL` | duration | `1m` | Rate limiter cleanup interval |
| `REQUEST_TIMEOUT` | duration | `30s` | Request processing timeout |
| `MAX_REQUEST_SIZE` | string | `1MB` | Maximum request body size |
| `CORS_ALLOWED_ORIGINS` | string | `*` | CORS allowed origins (comma-separated) |
| `CORS_ALLOWED_METHODS` | string | `GET,POST,PUT,DELETE,OPTIONS` | CORS allowed methods |
| `CORS_ALLOWED_HEADERS` | string | `*` | CORS allowed headers |

### Monitoring Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `METRICS_ENABLED` | bool | `true` | Enable metrics collection |
| `METRICS_PATH` | string | `/metrics` | Metrics endpoint path |
| `HEALTH_CHECK_INTERVAL` | duration | `30s` | Health check interval |
| `HEALTH_CHECK_TIMEOUT` | duration | `10s` | Health check timeout |

### Circuit Breaker Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `CIRCUIT_BREAKER_ENABLED` | bool | `true` | Enable circuit breaker |
| `CIRCUIT_BREAKER_MAX_FAILURES` | int | `5` | Maximum failures before opening |
| `CIRCUIT_BREAKER_RESET_TIMEOUT` | duration | `60s` | Reset timeout |
| `CIRCUIT_BREAKER_SUCCESS_THRESHOLD` | int | `3` | Success threshold to close |

### Retry Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `RETRY_ENABLED` | bool | `true` | Enable retry mechanism |
| `RETRY_MAX_ATTEMPTS` | int | `3` | Maximum retry attempts |
| `RETRY_INITIAL_DELAY` | duration | `1s` | Initial retry delay |
| `RETRY_MAX_DELAY` | duration | `30s` | Maximum retry delay |
| `RETRY_MULTIPLIER` | float | `2.0` | Backoff multiplier |
| `RETRY_JITTER` | bool | `true` | Enable jitter |

## Configuration File Format

### .env File Example

```bash
# Server Configuration
PORT=8080
LOG_LEVEL=info
LOG_FORMAT=json

# Database Configuration
DATABASE_URL=postgres://cryptopulse:password@localhost:5432/cryptopulse?sslmode=disable
DATABASE_MAX_OPEN_CONNS=25
DATABASE_MAX_IDLE_CONNS=5

# dYdX Configuration
DYDX_NETWORK=testnet
INDEXER_URL=https://indexer.v4testnet.dydx.exchange
RPC_URL=https://test-dydx-rpc.kingnodes.com:443
CHAIN_ID=dydx-testnet-4

# Wallet Configuration
MNEMONIC=your twelve word mnemonic phrase goes here for wallet access
WALLET_HD_PATH=m/44'/118'/0'/0/0

# Security Configuration
API_KEY=your-secure-api-key-here
RATE_LIMIT_REQUESTS_PER_MINUTE=100
RATE_LIMIT_BURST=20

# Monitoring Configuration
METRICS_ENABLED=true
HEALTH_CHECK_INTERVAL=30s
```

### YAML Configuration (Optional)

```yaml
# config.yaml
server:
  port: 8080
  host: "0.0.0.0"
  timeouts:
    read: "30s"
    write: "30s"
    idle: "120s"
    shutdown: "30s"

database:
  url: "postgres://cryptopulse:password@localhost:5432/cryptopulse?sslmode=disable"
  pool:
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: "5m"
  timeouts:
    query: "30s"

dydx:
  network: "testnet"
  indexer_url: "https://indexer.v4testnet.dydx.exchange"
  rpc_url: "https://test-dydx-rpc.kingnodes.com:443"
  chain_id: "dydx-testnet-4"
  gas:
    price: "0.025"
    adjustment: 1.5

wallet:
  mnemonic: "your twelve word mnemonic phrase goes here for wallet access"
  hd_path: "m/44'/118'/0'/0/0"
  prefix: "dydx"

security:
  api_key: "your-secure-api-key-here"
  rate_limit:
    requests_per_minute: 100
    burst: 20
    cleanup_interval: "1m"
  cors:
    allowed_origins: ["*"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers: ["*"]

monitoring:
  metrics:
    enabled: true
    path: "/metrics"
  health_check:
    interval: "30s"
    timeout: "10s"

circuit_breaker:
  enabled: true
  max_failures: 5
  reset_timeout: "60s"
  success_threshold: 3

retry:
  enabled: true
  max_attempts: 3
  initial_delay: "1s"
  max_delay: "30s"
  multiplier: 2.0
  jitter: true

logging:
  level: "info"
  format: "json"
  output: "stdout"
```

## Environment-Specific Configuration

### Development Environment

```bash
# .env.development
LOG_LEVEL=debug
LOG_FORMAT=text
DYDX_NETWORK=testnet
METRICS_ENABLED=true
CIRCUIT_BREAKER_ENABLED=false
RETRY_MAX_ATTEMPTS=1
```

### Staging Environment

```bash
# .env.staging
LOG_LEVEL=info
LOG_FORMAT=json
DYDX_NETWORK=testnet
DATABASE_SSL_MODE=require
RATE_LIMIT_REQUESTS_PER_MINUTE=50
```

### Production Environment

```bash
# .env.production
LOG_LEVEL=warn
LOG_FORMAT=json
DYDX_NETWORK=mainnet
DATABASE_SSL_MODE=verify-full
RATE_LIMIT_REQUESTS_PER_MINUTE=1000
CORS_ALLOWED_ORIGINS=https://app.cryptopulse.com
```

## Validation Rules

### Required Variables

The following environment variables are required and must be set:

- `DATABASE_URL` or (`DATABASE_HOST`, `DATABASE_NAME`, `DATABASE_USER`, `DATABASE_PASSWORD`)
- `MNEMONIC`
- `API_KEY`

### Format Validation

- **Duration**: Must be valid Go duration format (e.g., `30s`, `5m`, `1h`)
- **URL**: Must be valid HTTP/HTTPS URL
- **Mnemonic**: Must be valid BIP39 mnemonic phrase (12 or 24 words)
- **API Key**: Must be at least 32 characters long
- **Chain ID**: Must match the network configuration

### Value Constraints

- `PORT`: 1-65535
- `DATABASE_MAX_OPEN_CONNS`: 1-1000
- `DATABASE_MAX_IDLE_CONNS`: 1-100
- `RATE_LIMIT_REQUESTS_PER_MINUTE`: 1-10000
- `RATE_LIMIT_BURST`: 1-1000
- `GAS_ADJUSTMENT`: 1.0-10.0

## Configuration Loading

### Go Code Example

```go
package config

import (
    "os"
    "strconv"
    "time"
)

type Config struct {
    // Server configuration
    Port         string        `env:"PORT" default:"8080"`
    Host         string        `env:"HOST" default:"0.0.0.0"`
    LogLevel     string        `env:"LOG_LEVEL" default:"info"`
    
    // Database configuration
    DatabaseURL  string        `env:"DATABASE_URL" required:"true"`
    
    // dYdX configuration
    Network      string        `env:"DYDX_NETWORK" default:"testnet"`
    IndexerURL   string        `env:"INDEXER_URL" default:"https://indexer.v4testnet.dydx.exchange"`
    
    // Security configuration
    APIKey       string        `env:"API_KEY" required:"true"`
    
    // Timeouts
    RequestTimeout time.Duration `env:"REQUEST_TIMEOUT" default:"30s"`
}

func Load() (*Config, error) {
    cfg := &Config{}
    
    // Load from environment variables
    if err := loadFromEnv(cfg); err != nil {
        return nil, err
    }
    
    // Validate configuration
    if err := validate(cfg); err != nil {
        return nil, err
    }
    
    return cfg, nil
}
```

## Troubleshooting

### Common Configuration Issues

1. **Invalid Database URL**:
   ```
   Error: failed to connect to database: invalid connection string
   Solution: Check DATABASE_URL format and credentials
   ```

2. **Invalid Mnemonic**:
   ```
   Error: failed to derive wallet: invalid mnemonic
   Solution: Verify mnemonic is valid BIP39 phrase
   ```

3. **Network Mismatch**:
   ```
   Error: chain ID mismatch
   Solution: Ensure CHAIN_ID matches DYDX_NETWORK
   ```

4. **Permission Denied**:
   ```
   Error: bind: permission denied
   Solution: Use port > 1024 or run with appropriate privileges
   ```

### Configuration Validation

Use the built-in configuration validation:

```bash
# Validate configuration without starting the service
./cryptopulse --validate-config

# Check specific configuration values
./cryptopulse --config-check database
./cryptopulse --config-check wallet
./cryptopulse --config-check security
```

### Environment Variable Debugging

```bash
# Print all environment variables
env | grep -E "(DATABASE|DYDX|WALLET|API)" | sort

# Check specific variable
echo $DATABASE_URL
echo $MNEMONIC | wc -w  # Should output 12 or 24
```
