# dYdX Order Routing Service - Deployment Guide

This guide covers deployment of the dYdX Order Routing Service to preprod and production environments using Docker.

## Quick Start

### Prerequisites

- Docker 20.10+
- Docker Compose 2.0+
- Make
- Git

### Environment Setup

1. **Preprod Deployment**:
   ```bash
   # Copy and configure environment
   cp .env.preprod.example .env.preprod
   # Edit .env.preprod with your testnet configuration
   
   # Deploy
   make deploy-preprod
   ```

2. **Production Deployment**:
   ```bash
   # Copy and configure environment
   cp .env.prod.example .env.prod
   # Edit .env.prod with your mainnet configuration
   
   # Deploy (with confirmation prompt)
   make deploy-prod
   ```

## Docker Images

### Development (`Dockerfile.dev`)
- Based on `golang:1.23-alpine`
- Includes Air for hot reloading
- Debug symbols enabled
- Suitable for local development

### Preprod (`Dockerfile.preprod`)
- Multi-stage build with Alpine Linux
- Race detection enabled
- Optimized for testing and validation
- Includes debugging capabilities
- Non-root user for security

### Production (`Dockerfile.prod`)
- Multi-stage build with distroless base
- Static binary compilation
- Maximum security and minimal attack surface
- No shell or package manager
- Optimized for performance and security

## Environment Configuration

### Required Environment Variables

#### Database
- `DB_PASSWORD`: Secure database password
- `DATABASE_MAX_OPEN_CONNS`: Connection pool size
- `DATABASE_MAX_IDLE_CONNS`: Idle connections

#### dYdX Network
- `DYDX_NETWORK`: `testnet` for preprod, `mainnet` for production
- `INDEXER_URL`: dYdX Indexer API endpoint
- `RPC_URL`: Cosmos RPC endpoint
- `CHAIN_ID`: Blockchain chain ID

#### Security
- `API_KEY`: Secure API key (min 32 chars for preprod, 64 for prod)
- `MNEMONIC`: Wallet mnemonic phrase
- `RATE_LIMIT_REQUESTS_PER_MINUTE`: Rate limiting configuration

#### Monitoring
- `GRAFANA_PASSWORD`: Grafana admin password
- `REDIS_PASSWORD`: Redis password

### Security Considerations

1. **Never commit real credentials** to version control
2. **Use dedicated wallets** for each environment
3. **Rotate API keys** regularly
4. **Enable SSL/TLS** in production
5. **Restrict CORS origins** to your domain only

## Deployment Commands

### Preprod Environment

```bash
# Build preprod image
make docker-build-preprod

# Deploy preprod stack
make deploy-preprod

# View logs
make logs-preprod

# Stop preprod
make stop-preprod

# Health check
curl http://localhost:8080/health
```

### Production Environment

```bash
# Build production image
make docker-build-prod

# Deploy production stack (with confirmation)
make deploy-prod

# View logs
make logs-prod

# Stop production (with confirmation)
make stop-prod

# Health check
curl http://localhost:8080/health
```

## Monitoring Stack

Both preprod and production include:

- **Prometheus**: Metrics collection
- **Grafana**: Visualization and dashboards
- **Redis**: Caching and rate limiting
- **PostgreSQL**: Primary database

### Accessing Monitoring

- **Grafana**: http://localhost:3000 (admin/password from env)
- **Prometheus**: http://localhost:9090

## Health Checks

The application provides multiple health check endpoints:

- `/health`: Overall application health
- `/health/live`: Kubernetes liveness probe
- `/health/ready`: Kubernetes readiness probe

### Docker Health Checks

All containers include health checks:
- Application: HTTP health endpoint
- PostgreSQL: `pg_isready`
- Redis: Connection test
- Prometheus/Grafana: HTTP endpoints

## Scaling and High Availability

### Production Scaling

The production Docker Compose includes:
- **2 application replicas** by default
- **Rolling updates** with zero downtime
- **Resource limits** and reservations
- **Restart policies** for fault tolerance

### Load Balancing

Production includes Nginx for:
- SSL termination
- Load balancing between app instances
- Static file serving
- Security headers

## Backup and Recovery

### Database Backups

Production includes automated backups:
- **Daily backups** at 2 AM
- **30-day retention**
- **S3 storage** (configure AWS credentials)

### Manual Backup

```bash
# Create manual backup
docker exec cryptopulse-postgres-prod pg_dump -U cryptopulse cryptopulse > backup.sql

# Restore from backup
docker exec -i cryptopulse-postgres-prod psql -U cryptopulse cryptopulse < backup.sql
```

## Troubleshooting

### Common Issues

1. **Port conflicts**: Ensure ports 8080, 3000, 9090 are available
2. **Environment variables**: Check `.env.preprod` or `.env.prod` files
3. **Database connection**: Verify PostgreSQL is healthy
4. **Memory limits**: Increase if containers are OOMKilled

### Debug Commands

```bash
# Check container status
docker ps

# View container logs
docker logs cryptopulse-preprod
docker logs cryptopulse-prod

# Execute shell in container (preprod only)
docker exec -it cryptopulse-preprod sh

# Check application health
curl -v http://localhost:8080/health

# Check metrics
curl http://localhost:8080/metrics
```

### Log Analysis

```bash
# Follow application logs
make logs-preprod  # or logs-prod

# Filter logs by level
docker logs cryptopulse-preprod 2>&1 | grep ERROR

# Check database logs
docker logs cryptopulse-postgres-preprod
```

## Security Hardening

### Production Security

1. **Network isolation**: Services communicate via internal network
2. **Non-root users**: All containers run as non-root
3. **Read-only filesystems**: Where possible
4. **Resource limits**: Prevent resource exhaustion
5. **Security scanning**: Regular image vulnerability scans

### SSL/TLS Configuration

For production, configure SSL certificates:

```bash
# Place certificates in nginx/ssl/
mkdir -p nginx/ssl
cp your-cert.pem nginx/ssl/cert.pem
cp your-key.pem nginx/ssl/key.pem
```

## Maintenance

### Updates

```bash
# Update to new version
git pull
make deploy-preprod  # Test first
make deploy-prod     # Then production
```

### Cleanup

```bash
# Remove unused images
docker image prune -f

# Remove unused volumes
docker volume prune -f

# Full cleanup (careful!)
docker system prune -af
```

## Support

For deployment issues:
1. Check the logs first
2. Verify environment configuration
3. Test in preprod before production
4. Consult the main documentation in `docs/`
