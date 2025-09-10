# dYdX Testing Environment Guide

This guide provides detailed instructions for testing the dYdX Order Routing Service in the dYdX V4 testnet environment.

## Table of Contents

- [Prerequisites](#prerequisites)
- [dYdX V4 Testnet Setup](#dydx-v4-testnet-setup)
- [Wallet Configuration](#wallet-configuration)
- [Service Configuration](#service-configuration)
- [Testing Procedures](#testing-procedures)
- [API Testing](#api-testing)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### System Requirements

- Go 1.23+
- Docker and Docker Compose
- curl or Postman for API testing
- Git

### dYdX V4 Testnet Information

- **Network**: dYdX V4 Testnet
- **Chain ID**: `dydx-testnet-4`
- **Indexer API**: `https://indexer.v4testnet.dydx.exchange`
- **RPC Endpoint**: `https://test-dydx-rpc.kingnodes.com:443`
- **Block Explorer**: `https://testnet.mintscan.io/dydx-testnet`
- **Faucet**: Available through dYdX testnet interface

## dYdX V4 Testnet Setup

### 1. Access dYdX V4 Testnet

1. **Visit the dYdX V4 Testnet Interface**:
   ```
   https://v4.testnet.dydx.exchange/
   ```

2. **Connect Your Wallet**:
   - Use MetaMask or Keplr wallet
   - Add dYdX V4 testnet network if not already added
   - Network details:
     - Network Name: dYdX V4 Testnet
     - RPC URL: https://test-dydx-rpc.kingnodes.com:443
     - Chain ID: dydx-testnet-4
     - Currency Symbol: DYDX
     - Block Explorer: https://testnet.mintscan.io/dydx-testnet

### 2. Get Testnet Tokens

1. **Request Testnet DYDX**:
   - Use the built-in faucet in the dYdX testnet interface
   - Or visit: `https://faucet.v4testnet.dydx.exchange/`
   - You'll need testnet DYDX for gas fees and trading

2. **Get Testnet USDC**:
   - The testnet provides USDC for trading
   - Use the deposit feature in the testnet interface
   - Minimum amount: $1000 USDC equivalent

### 3. Create Test Account

1. **Generate a New Wallet** (recommended for testing):
   ```bash
   # Using dYdX CLI or any Cosmos wallet generator
   # Or create through the web interface
   ```

2. **Export Mnemonic**:
   - Save the 12 or 24-word mnemonic phrase
   - This will be used in your service configuration
   - **NEVER use mainnet wallets for testing**

## Wallet Configuration

### 1. Prepare Test Wallet

Create a dedicated testnet wallet:

```bash
# Example mnemonic (DO NOT USE IN PRODUCTION)
# Generate your own using: https://iancoleman.io/bip39/
TESTNET_MNEMONIC="abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
```

### 2. Fund the Wallet

1. **Get the wallet address**:
   ```bash
   # Start the service temporarily to see the wallet address
   make run
   # Look for log: "Wallet initialized" with address
   ```

2. **Fund with testnet tokens**:
   - Send testnet DYDX to the address for gas fees
   - Deposit testnet USDC for trading

### 3. Verify Wallet Setup

Check wallet balance using dYdX Indexer API:

```bash
# Replace with your actual wallet address
WALLET_ADDRESS="dydx1your-wallet-address-here"

curl -X GET "https://indexer.v4testnet.dydx.exchange/v4/addresses/${WALLET_ADDRESS}" \
  -H "accept: application/json"
```

## Service Configuration

### 1. Environment Setup

Create testnet configuration:

```bash
# Copy the local environment file
cp .env.local .env.testnet

# Edit .env.testnet with testnet-specific values
```

### 2. Testnet Configuration

Update `.env.testnet` with the following values:

```bash
# Server Configuration
PORT=8080
LOG_LEVEL=debug
LOG_FORMAT=text
ENVIRONMENT=testnet

# Database Configuration (use local PostgreSQL)
DATABASE_URL=postgres://cryptopulse:password@localhost:5432/cryptopulse?sslmode=disable

# dYdX V4 Testnet Configuration
DYDX_NETWORK=testnet
INDEXER_URL=https://indexer.v4testnet.dydx.exchange
RPC_URL=https://test-dydx-rpc.kingnodes.com:443
CHAIN_ID=dydx-testnet-4

# Wallet Configuration - TESTNET ONLY
MNEMONIC="your testnet mnemonic phrase here twelve words"
WALLET_HD_PATH="m/44'/118'/0'/0/0"

# Security Configuration
API_KEY=testnet-api-key-for-testing-only
RATE_LIMIT_REQUESTS_PER_MINUTE=100
RATE_LIMIT_BURST=20

# Monitoring Configuration
METRICS_ENABLED=true
HEALTH_CHECK_INTERVAL=30s
```

### 3. Start the Service

```bash
# Start development environment
make dev-up

# Run database migrations
make migrate-up

# Start the service with testnet configuration
set -a && source .env.testnet && set +a && make run
```

## Testing Procedures

### 1. Health Check

Verify the service is running:

```bash
curl -X GET http://localhost:8080/health
```

Expected response:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:00:00Z",
  "version": "1.0.0",
  "uptime": "5m30s",
  "components": [
    {
      "name": "database",
      "status": "healthy",
      "message": "Database is responsive"
    },
    {
      "name": "dydx_indexer",
      "status": "healthy",
      "message": "dYdX Indexer API is accessible"
    }
  ]
}
```

### 2. Wallet Verification

Check if the wallet is properly initialized:

```bash
# Check service logs for wallet address
# Should see: "Wallet initialized" with dydx address
```

### 3. Market Data Verification

Test connection to dYdX Indexer:

```bash
# Get available markets
curl -X GET "https://indexer.v4testnet.dydx.exchange/v4/perpetualMarkets" \
  -H "accept: application/json"

# Get specific market (BTC-USD)
curl -X GET "https://indexer.v4testnet.dydx.exchange/v4/perpetualMarkets/BTC-USD" \
  -H "accept: application/json"
```

## API Testing

### 1. Authentication Setup

All API requests require the API key:

```bash
API_KEY="testnet-api-key-for-testing-only"
BASE_URL="http://localhost:8080"
```

### 2. Place Test Orders

#### Market Buy Order

```bash
curl -X POST "${BASE_URL}/api/v1/orders" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "clientId": "test-market-buy-001",
    "market": "BTC-USD",
    "side": "BUY",
    "type": "MARKET",
    "size": "0.001",
    "timeInForce": "IOC"
  }'
```

#### Limit Sell Order

```bash
curl -X POST "${BASE_URL}/api/v1/orders" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "clientId": "test-limit-sell-001",
    "market": "BTC-USD",
    "side": "SELL",
    "type": "LIMIT",
    "size": "0.001",
    "price": "50000.0",
    "timeInForce": "GTT",
    "goodTilBlock": 1000000
  }'
```

### 3. Order Management

#### Get Order Status

```bash
# Replace with actual order ID from place order response
ORDER_ID="123e4567-e89b-12d3-a456-426614174000"

curl -X GET "${BASE_URL}/api/v1/orders/${ORDER_ID}" \
  -H "X-API-Key: ${API_KEY}"
```

#### Cancel Order

```bash
curl -X DELETE "${BASE_URL}/api/v1/orders/${ORDER_ID}" \
  -H "X-API-Key: ${API_KEY}"
```

#### Get Order History

```bash
curl -X GET "${BASE_URL}/api/v1/orders?market=BTC-USD&status=FILLED&limit=10" \
  -H "X-API-Key: ${API_KEY}"
```

### 4. Position Management

#### Get Positions

```bash
curl -X GET "${BASE_URL}/api/v1/positions" \
  -H "X-API-Key: ${API_KEY}"
```

#### Close Position

```bash
curl -X POST "${BASE_URL}/api/v1/positions/BTC-USD/close" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "clientId": "close-btc-position-001"
  }'
```

## Advanced Testing Scenarios

### 1. Error Handling Tests

#### Invalid Market

```bash
curl -X POST "${BASE_URL}/api/v1/orders" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "clientId": "test-invalid-market",
    "market": "INVALID-USD",
    "side": "BUY",
    "type": "MARKET",
    "size": "0.001"
  }'
```

#### Insufficient Funds

```bash
curl -X POST "${BASE_URL}/api/v1/orders" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d '{
    "clientId": "test-insufficient-funds",
    "market": "BTC-USD",
    "side": "BUY",
    "type": "MARKET",
    "size": "1000.0"
  }'
```

### 2. Rate Limiting Tests

```bash
# Send multiple requests quickly to test rate limiting
for i in {1..10}; do
  curl -X GET "${BASE_URL}/health" \
    -H "X-API-Key: ${API_KEY}" &
done
wait
```

### 3. Concurrent Order Tests

```bash
# Place multiple orders concurrently
for i in {1..5}; do
  curl -X POST "${BASE_URL}/api/v1/orders" \
    -H "Content-Type: application/json" \
    -H "X-API-Key: ${API_KEY}" \
    -d "{
      \"clientId\": \"concurrent-test-${i}\",
      \"market\": \"BTC-USD\",
      \"side\": \"BUY\",
      \"type\": \"LIMIT\",
      \"size\": \"0.001\",
      \"price\": \"45000.0\"
    }" &
done
wait
```

## Monitoring and Debugging

### 1. Service Logs

Monitor service logs for debugging:

```bash
# Follow logs in real-time
tail -f /path/to/service/logs

# Or if running with make dev
# Logs will appear in the terminal
```

### 2. Database Inspection

Check order data in the database:

```bash
# Connect to database
make db-shell

# Query orders
SELECT id, client_id, market, side, type, size, price, status, created_at 
FROM orders 
ORDER BY created_at DESC 
LIMIT 10;

# Query order history
SELECT o.client_id, o.market, h.old_status, h.new_status, h.created_at
FROM orders o
JOIN order_status_history h ON o.id = h.order_id
ORDER BY h.created_at DESC
LIMIT 20;
```

### 3. dYdX Indexer Verification

Verify orders appear in dYdX Indexer:

```bash
# Get orders for your address
WALLET_ADDRESS="your-wallet-address"
curl -X GET "https://indexer.v4testnet.dydx.exchange/v4/orders?address=${WALLET_ADDRESS}" \
  -H "accept: application/json"
```

## Troubleshooting

### Common Issues

1. **Wallet Not Funded**:
   ```
   Error: insufficient funds for gas
   Solution: Fund wallet with testnet DYDX tokens
   ```

2. **Invalid Mnemonic**:
   ```
   Error: failed to derive wallet: invalid mnemonic
   Solution: Verify mnemonic is valid BIP39 phrase
   ```

3. **Network Connection Issues**:
   ```
   Error: failed to connect to dYdX Indexer
   Solution: Check INDEXER_URL and network connectivity
   ```

4. **Order Rejection**:
   ```
   Error: order rejected by dYdX
   Solution: Check order parameters, market status, and account balance
   ```

### Debug Commands

```bash
# Check wallet balance
curl "https://indexer.v4testnet.dydx.exchange/v4/addresses/${WALLET_ADDRESS}"

# Check market status
curl "https://indexer.v4testnet.dydx.exchange/v4/perpetualMarkets/BTC-USD"

# Test RPC connection
curl -X POST "https://test-dydx-rpc.kingnodes.com:443" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"status","params":[],"id":1}'

# Check service health
curl -v http://localhost:8080/health
```

### Log Analysis

Look for these key log messages:

```
✅ "Wallet initialized" - Wallet setup successful
✅ "Successfully connected to database" - Database connection OK
✅ "Starting server" - Service started successfully
❌ "Failed to place order" - Order placement issues
❌ "Database connection failed" - Database issues
❌ "dYdX API error" - External API issues
```

## Best Practices

1. **Use Dedicated Testnet Wallet**: Never use mainnet wallets for testing
2. **Start Small**: Begin with small order sizes (0.001 BTC)
3. **Monitor Logs**: Keep an eye on service logs during testing
4. **Test Error Cases**: Verify error handling works correctly
5. **Clean Up**: Cancel test orders and close positions after testing
6. **Document Issues**: Keep track of any bugs or unexpected behavior

## Next Steps

After successful testnet testing:

1. **Performance Testing**: Test with higher order volumes
2. **Integration Testing**: Test with your frontend application
3. **Security Review**: Audit configuration and access controls
4. **Mainnet Preparation**: Prepare production configuration
5. **Monitoring Setup**: Configure production monitoring and alerting

For production deployment, see the [Deployment Guide](../DEPLOYMENT.md).
