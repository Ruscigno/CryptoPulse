# dYdX Order Routing Service - Deployment Guide

This guide covers the deployment of the dYdX Order Routing Service MVP in various environments.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Environment Configuration](#environment-configuration)
- [Local Development](#local-development)
- [Docker Deployment](#docker-deployment)
- [Production Deployment](#production-deployment)
- [Monitoring and Logging](#monitoring-and-logging)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### System Requirements

- **Go**: Version 1.21 or higher
- **Docker**: Version 20.10 or higher
- **Docker Compose**: Version 2.0 or higher
- **PostgreSQL**: Version 14 or higher
- **Redis**: Version 6.0 or higher (optional, for caching)

### External Dependencies

- **dYdX V4 Network Access**: 
  - Testnet: `https://indexer.v4testnet.dydx.exchange`
  - Mainnet: `https://indexer.dydx.trade`
- **Cosmos SDK Compatible Wallet**: BIP39 mnemonic phrase required

## Environment Configuration

### Required Environment Variables

Create a `.env` file based on `.env.example`:

```bash
# Server Configuration
PORT=8080
LOG_LEVEL=info
API_KEY=your-secure-api-key-here

# Database Configuration
DATABASE_URL=postgres://cryptopulse:password@localhost:5432/cryptopulse?sslmode=disable
DATABASE_MAX_OPEN_CONNS=25
DATABASE_MAX_IDLE_CONNS=5
DATABASE_CONN_MAX_LIFETIME=5m

# dYdX Configuration
DYDX_NETWORK=testnet
INDEXER_URL=https://indexer.v4testnet.dydx.exchange
RPC_URL=https://test-dydx-rpc.kingnodes.com:443
CHAIN_ID=dydx-testnet-4

# Wallet Configuration
MNEMONIC=your-wallet-mnemonic-phrase-here
WALLET_HD_PATH=m/44'/118'/0'/0/0

# Security Configuration
RATE_LIMIT_REQUESTS_PER_MINUTE=100
RATE_LIMIT_BURST=20
REQUEST_TIMEOUT=30s
MAX_REQUEST_SIZE=1MB

# Monitoring Configuration
METRICS_ENABLED=true
HEALTH_CHECK_INTERVAL=30s
```

### Security Considerations

1. **API Key**: Generate a strong, unique API key for each environment
2. **Mnemonic**: Use a dedicated wallet for the service, never reuse personal wallets
3. **Database**: Use strong passwords and enable SSL in production
4. **Network**: Restrict access to internal networks where possible

## Local Development

### Quick Start

1. **Clone and Setup**:
   ```bash
   git clone <repository-url>
   cd CryptoPulse
   cp .env.example .env.local
   # Edit .env.local with your configuration
   ```

2. **Start Dependencies**:
   ```bash
   make dev-up
   ```

3. **Run Database Migrations**:
   ```bash
   make migrate-up
   ```

4. **Start the Service**:
   ```bash
   make dev
   ```

5. **Verify Installation**:
   ```bash
   curl http://localhost:8080/health
   ```

### Development Workflow

- **Hot Reload**: The service uses Air for hot reloading during development
- **Database Reset**: Use `make dev-clean` to reset the database
- **Logs**: View logs with `make dev-logs`
- **Tests**: Run tests with `make test-all`

## Docker Deployment

### Building the Image

```bash
# Build development image
docker build -f Dockerfile.dev -t cryptopulse:dev .

# Build production image
docker build -t cryptopulse:latest .
```

### Docker Compose Deployment

1. **Production Compose File** (`docker-compose.prod.yml`):
   ```yaml
   version: '3.8'
   
   services:
     app:
       image: cryptopulse:latest
       ports:
         - "8080:8080"
       environment:
         - DATABASE_URL=postgres://cryptopulse:${DB_PASSWORD}@postgres:5432/cryptopulse?sslmode=require
         - API_KEY=${API_KEY}
         - MNEMONIC=${MNEMONIC}
       depends_on:
         - postgres
         - redis
       restart: unless-stopped
       healthcheck:
         test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
         interval: 30s
         timeout: 10s
         retries: 3
   
     postgres:
       image: postgres:14-alpine
       environment:
         - POSTGRES_DB=cryptopulse
         - POSTGRES_USER=cryptopulse
         - POSTGRES_PASSWORD=${DB_PASSWORD}
       volumes:
         - postgres_data:/var/lib/postgresql/data
         - ./pkg/database/migrations:/docker-entrypoint-initdb.d
       restart: unless-stopped
   
     redis:
       image: redis:6-alpine
       restart: unless-stopped
   
   volumes:
     postgres_data:
   ```

2. **Deploy**:
   ```bash
   # Set environment variables
   export DB_PASSWORD=your-secure-db-password
   export API_KEY=your-secure-api-key
   export MNEMONIC=your-wallet-mnemonic
   
   # Deploy
   docker compose -f docker-compose.prod.yml up -d
   ```

## Production Deployment

### Kubernetes Deployment

1. **Create Namespace**:
   ```bash
   kubectl create namespace cryptopulse
   ```

2. **Create Secrets**:
   ```bash
   kubectl create secret generic cryptopulse-secrets \
     --from-literal=api-key=your-secure-api-key \
     --from-literal=mnemonic=your-wallet-mnemonic \
     --from-literal=db-password=your-db-password \
     -n cryptopulse
   ```

3. **Deploy Database**:
   ```yaml
   # postgres-deployment.yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: postgres
     namespace: cryptopulse
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: postgres
     template:
       metadata:
         labels:
           app: postgres
       spec:
         containers:
         - name: postgres
           image: postgres:14-alpine
           env:
           - name: POSTGRES_DB
             value: cryptopulse
           - name: POSTGRES_USER
             value: cryptopulse
           - name: POSTGRES_PASSWORD
             valueFrom:
               secretKeyRef:
                 name: cryptopulse-secrets
                 key: db-password
           ports:
           - containerPort: 5432
           volumeMounts:
           - name: postgres-storage
             mountPath: /var/lib/postgresql/data
         volumes:
         - name: postgres-storage
           persistentVolumeClaim:
             claimName: postgres-pvc
   ```

4. **Deploy Application**:
   ```yaml
   # app-deployment.yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: cryptopulse
     namespace: cryptopulse
   spec:
     replicas: 3
     selector:
       matchLabels:
         app: cryptopulse
     template:
       metadata:
         labels:
           app: cryptopulse
       spec:
         containers:
         - name: cryptopulse
           image: cryptopulse:latest
           ports:
           - containerPort: 8080
           env:
           - name: DATABASE_URL
             value: postgres://cryptopulse:$(DB_PASSWORD)@postgres:5432/cryptopulse?sslmode=require
           - name: API_KEY
             valueFrom:
               secretKeyRef:
                 name: cryptopulse-secrets
                 key: api-key
           - name: MNEMONIC
             valueFrom:
               secretKeyRef:
                 name: cryptopulse-secrets
                 key: mnemonic
           - name: DB_PASSWORD
             valueFrom:
               secretKeyRef:
                 name: cryptopulse-secrets
                 key: db-password
           livenessProbe:
             httpGet:
               path: /health
               port: 8080
             initialDelaySeconds: 30
             periodSeconds: 10
           readinessProbe:
             httpGet:
               path: /health
               port: 8080
             initialDelaySeconds: 5
             periodSeconds: 5
   ```

### Load Balancer and Ingress

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: cryptopulse-ingress
  namespace: cryptopulse
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/rate-limit: "100"
spec:
  tls:
  - hosts:
    - api.cryptopulse.com
    secretName: cryptopulse-tls
  rules:
  - host: api.cryptopulse.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: cryptopulse-service
            port:
              number: 8080
```

## Monitoring and Logging

### Health Checks

The service provides several health check endpoints:

- `/health` - Overall health status
- `/health/live` - Liveness probe
- `/health/ready` - Readiness probe

### Metrics Collection

If using Prometheus:

```yaml
# prometheus-config.yaml
scrape_configs:
  - job_name: 'cryptopulse'
    static_configs:
      - targets: ['cryptopulse:8080']
    metrics_path: /metrics
    scrape_interval: 15s
```

### Logging Configuration

Configure structured logging in production:

```bash
LOG_LEVEL=info
LOG_FORMAT=json
LOG_OUTPUT=stdout
```

## Troubleshooting

### Common Issues

1. **Database Connection Failed**:
   - Check DATABASE_URL format
   - Verify database is running and accessible
   - Check firewall rules

2. **Wallet Initialization Failed**:
   - Verify MNEMONIC is valid BIP39 phrase
   - Check HD_PATH format
   - Ensure wallet has sufficient funds for gas

3. **dYdX API Errors**:
   - Verify INDEXER_URL and RPC_URL are correct
   - Check network connectivity
   - Validate CHAIN_ID matches the network

4. **High Memory Usage**:
   - Check for connection leaks
   - Monitor goroutine count
   - Review database connection pool settings

### Debug Mode

Enable debug logging:

```bash
LOG_LEVEL=debug
```

### Performance Tuning

1. **Database Connections**:
   ```bash
   DATABASE_MAX_OPEN_CONNS=25
   DATABASE_MAX_IDLE_CONNS=5
   DATABASE_CONN_MAX_LIFETIME=5m
   ```

2. **Rate Limiting**:
   ```bash
   RATE_LIMIT_REQUESTS_PER_MINUTE=100
   RATE_LIMIT_BURST=20
   ```

3. **Timeouts**:
   ```bash
   REQUEST_TIMEOUT=30s
   DATABASE_QUERY_TIMEOUT=10s
   ```

### Support

For additional support:

1. Check the [API Documentation](./api/openapi.yaml)
2. Review [Database Schema](./database-schema.md)
3. Consult [Configuration Reference](./configuration.md)
4. Submit issues to the project repository
