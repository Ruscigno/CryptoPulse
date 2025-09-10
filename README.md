# CryptoPulse - dYdX Order Routing Service

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org)
[![Docker](https://img.shields.io/badge/Docker-20.10+-blue.svg)](https://docker.com)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Status](https://img.shields.io/badge/Status-Implementation%20Complete-yellow.svg)](#status)

A high-performance Go-Kit based microservice for routing orders to the dYdX V4 decentralized exchange. Built for AI trading systems requiring reliable, fast, and secure order execution.

## 🚀 Features

- **Complete dYdX V4 Integration** - Place, cancel, and manage orders on dYdX
- **Go-Kit Architecture** - Scalable microservice with proper separation of concerns
- **Production Ready** - Docker containers, monitoring, security middleware
- **Comprehensive API** - RESTful endpoints for all trading operations
- **Real-time Monitoring** - Prometheus metrics, Grafana dashboards, health checks
- **Security First** - API key authentication, rate limiting, input validation
- **Database Integration** - PostgreSQL with order tracking and history
- **Error Resilience** - Circuit breaker pattern, exponential backoff retry with jitter

## 📋 Table of Contents

- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [API Endpoints](#api-endpoints)
- [Configuration](#configuration)
- [Development](#development)
- [Testing](#testing)
- [Deployment](#deployment)
- [Documentation](#documentation)
- [Status](#status)

## 🏃 Quick Start

### Prerequisites

- Go 1.23+
- Docker & Docker Compose
- Make

### 1. Clone and Setup

```bash
git clone https://github.com/Ruscigno/stock-screener.git
cd CryptoPulse

# Start development environment
make dev-up

# Run database migrations
make migrate-up

# Start the service
make dev
```

### 2. Configure Environment

```bash
# Copy environment template
cp .env.example .env.local

# Edit .env.local with your configuration
# - Add your testnet mnemonic
# - Configure dYdX endpoints
# - Set API keys
```

### 3. Test the Service

```bash
# Health check
curl http://localhost:8080/health

# Place a test order (requires API key)
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "clientId": "test-order-001",
    "market": "BTC-USD",
    "side": "BUY",
    "type": "MARKET",
    "size": "0.001"
  }'
```

## 🏗️ Architecture

Built using Go-Kit microservice patterns:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │───▶│  Transport Layer │───▶│ Endpoint Layer  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   PostgreSQL    │◀───│ Repository Layer │◀───│ Service Layer   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   dYdX Chain    │◀───│ Transaction Layer│◀───│ Wallet Layer    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Key Components

- **Service Layer** (`pkg/service/`) - Business logic and order management
- **Transport Layer** (`pkg/transport/`) - HTTP handlers and middleware
- **Repository Layer** (`pkg/repository/`) - Database operations
- **Wallet Layer** (`pkg/wallet/`) - Crypto wallet and key management
- **Transaction Layer** (`pkg/tx/`) - dYdX blockchain integration

## 🔌 API Endpoints

### Order Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/orders` | Place a new order |
| DELETE | `/api/v1/orders/{id}` | Cancel an order |
| GET | `/api/v1/orders/{id}` | Get order status |
| GET | `/api/v1/orders` | Get order history |

### Position Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/positions` | Get all positions |
| POST | `/api/v1/positions/{market}/close` | Close a position |

### System

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/metrics` | Prometheus metrics |

### Example Order Request

```json
{
  "clientId": "unique-order-id",
  "market": "BTC-USD",
  "side": "BUY",
  "type": "LIMIT",
  "size": "0.001",
  "price": "45000.0",
  "timeInForce": "GTT",
  "goodTilBlock": 1000000
}
```

## ⚙️ Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | - |
| `MNEMONIC` | Wallet mnemonic phrase | - |
| `RPC_URL` | dYdX RPC endpoint | - |
| `INDEXER_URL` | dYdX Indexer API URL | - |
| `API_KEY` | Service API key | - |

See [Configuration Guide](docs/configuration.md) for complete reference.

## 🛠️ Development

### Available Make Commands

```bash
make dev-up          # Start development environment
make dev-down        # Stop development environment
make dev             # Run service with hot reload
make test-unit       # Run unit tests
make test-integration # Run integration tests
make test-all        # Run all tests
make migrate-up      # Apply database migrations
make migrate-down    # Rollback migrations
make build           # Build binary
make docker-build    # Build Docker image
```

### Project Structure

```
├── cmd/main.go              # Application entry point
├── pkg/
│   ├── service/            # Business logic
│   ├── endpoint/           # Go-Kit endpoints
│   ├── transport/          # HTTP transport
│   ├── repository/         # Database layer
│   ├── wallet/             # Wallet management
│   ├── tx/                 # Transaction building
│   └── middleware/         # HTTP middleware
├── docs/                   # Documentation
├── tests/                  # Test suites
└── docker-compose.yml      # Development environment
```

## 🧪 Testing

### Unit Tests
```bash
make test-unit
```

### Integration Tests
```bash
INTEGRATION_TESTS=true make test-integration
```

### dYdX Testnet Testing
Follow the comprehensive [dYdX Testing Guide](docs/dydx-testing.md) for real testnet validation.

## 🚀 Deployment

### Development
```bash
make deploy-dev
```

### Preprod
```bash
make deploy-preprod
```

### Production
```bash
make deploy-prod  # Requires confirmation
```

See [Deployment Guide](DEPLOYMENT.md) for detailed instructions.

## 📚 Documentation

- [API Documentation](docs/api/openapi.yaml) - OpenAPI 3.0 specification
- [Configuration Guide](docs/configuration.md) - Environment setup
- [Database Schema](docs/database-schema.md) - Database design
- [dYdX Testing Guide](docs/dydx-testing.md) - Testnet testing
- [Deployment Guide](DEPLOYMENT.md) - Production deployment
- [Action Plan](docs/action-plan.md) - Implementation roadmap

## 📊 Status

🚧 **Implementation Complete - Testing Pending**

### ✅ Completed
- Complete service implementation
- Production Docker configurations
- Comprehensive documentation
- Unit and database tests
- Security middleware
- Monitoring setup

### ⏳ Pending
- dYdX testnet validation
- Performance testing
- Security audit
- Load testing

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Issues**: [GitHub Issues](https://github.com/Ruscigno/stock-screener/issues)
- **Documentation**: [docs/](docs/)
- **Testing**: [docs/dydx-testing.md](docs/dydx-testing.md)

---

**Built with ❤️ for AI trading systems**
