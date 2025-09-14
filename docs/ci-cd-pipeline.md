# CI/CD Pipeline Documentation

## Overview

The CryptoPulse CI/CD pipeline provides comprehensive testing and validation for the dYdX Order Routing Service. The pipeline is designed to ensure code quality, security, and reliability through multiple layers of testing.

## Pipeline Architecture

### Trigger Events
- **Push**: `main`, `develop`, `dydx` branches
- **Pull Request**: `main`, `develop` branches

### Pipeline Jobs

#### 1. Code Quality & Security
- **Purpose**: Static analysis and security scanning
- **Tools**: `go vet`, `staticcheck`, `gosec`
- **Duration**: ~2-3 minutes
- **Dependencies**: None (runs first)

#### 2. Unit Tests
- **Purpose**: Core business logic validation
- **Coverage**: Unit tests with race detection
- **Artifacts**: Coverage reports, HTML coverage
- **Duration**: ~1-2 minutes
- **Dependencies**: Code Quality

#### 3. OpenAPI Contract Tests ✨ **NEW**
- **Purpose**: API specification alignment validation
- **Coverage**: 
  - Endpoint existence verification
  - Request/response structure validation
  - Authentication requirement testing
  - HTTP method validation
  - Schema definition alignment
- **Artifacts**: Contract test results
- **Duration**: ~30-60 seconds
- **Dependencies**: Code Quality

#### 4. Build Tests
- **Purpose**: Cross-platform compilation validation
- **Coverage**: Linux amd64/arm64 builds, binary testing
- **Artifacts**: Container build artifacts
- **Duration**: ~2-3 minutes
- **Dependencies**: Code Quality

#### 5. Integration & E2E Tests ✨ **ENHANCED**
- **Purpose**: Full system validation with real dependencies
- **Services**: PostgreSQL 14, Redis 7
- **Coverage**:
  - Database integration tests
  - End-to-end API workflow tests
  - Real application startup and health checks
- **Artifacts**: Integration test results
- **Duration**: ~3-5 minutes
- **Dependencies**: Unit Tests, Contract Tests, Build Tests

#### 6. Docker Build Tests
- **Purpose**: Container build validation
- **Coverage**: Dev, preprod, production Docker images
- **Duration**: ~3-4 minutes
- **Dependencies**: Code Quality

#### 7. Security Scan
- **Purpose**: Vulnerability assessment with production/test separation
- **Tools**: Trivy scanner (dual-mode)
- **Coverage**:
  - Production dependencies scan (critical - blocks deployment)
  - All dependencies scan (informational - includes test libraries)
- **Artifacts**: SARIF security reports, detailed scan results, ignore file
- **Duration**: ~1-2 minutes
- **Dependencies**: Code Quality

#### 8. Test Summary ✨ **NEW**
- **Purpose**: Comprehensive test result reporting
- **Coverage**: 
  - Test result aggregation
  - Status dashboard generation
  - Artifact collection
  - Final pipeline status determination
- **Artifacts**: Test summary report (Markdown)
- **Duration**: ~30 seconds
- **Dependencies**: All test jobs

## Test Coverage Matrix

| Test Type | Coverage | Files Tested | Duration |
|-----------|----------|--------------|----------|
| Unit Tests | Business logic, validation, mocking | `tests/unit/` | ~1-2 min |
| Contract Tests | API specification alignment | `tests/api/` | ~30-60 sec |
| Integration Tests | Database, repositories, transactions | `tests/integration/` | ~2-3 min |
| E2E Tests | Full application workflows | `tests/e2e/` | ~1-2 min |
| Build Tests | Cross-platform compilation | Binary artifacts | ~2-3 min |
| Docker Tests | Container builds | Dockerfile variants | ~3-4 min |
| Security Tests | Vulnerability scanning (production + all deps) | Entire codebase | ~1-2 min |

## Environment Configuration

### Database Services
```yaml
postgres:
  image: postgres:14
  credentials:
    user: cryptopulse
    password: cryptopulse_test
    database: cryptopulse_test
  health_check: pg_isready

redis:
  image: redis:7-alpine
  health_check: redis-cli ping
```

### Environment Variables
```bash
# Application Configuration
PORT=8080
LOG_LEVEL=debug
API_KEY=test-api-key-for-ci-cd-pipeline

# Database Configuration
DATABASE_URL=postgres://cryptopulse:cryptopulse_test@localhost:5432/cryptopulse_test?sslmode=disable
REDIS_URL=redis://localhost:6379

# dYdX Configuration
MNEMONIC="abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
RPC_URL=https://test-dydx-rpc.kingnodes.com:443
INDEXER_URL=https://indexer.v4testnet.dydx.exchange
CHAIN_ID=dydx-testnet-4

# Test Configuration
INTEGRATION_TESTS=true
E2E_TESTS=true
CGO_ENABLED=1
```

## Artifacts Generated

### Test Reports
- **Coverage Report**: HTML and raw coverage data
- **Contract Test Results**: OpenAPI validation results
- **Integration Test Results**: Database and E2E test outputs
- **Test Summary Report**: Comprehensive pipeline status

### Build Artifacts
- **Container Binaries**: Linux amd64/arm64 executables
- **Security Reports**: SARIF vulnerability assessments

### Deployment Artifacts
- **Docker Images**: Dev, preprod, production variants

## Pipeline Status Indicators

### Success Criteria
All critical tests must pass:
- ✅ Unit Tests
- ✅ OpenAPI Contract Tests  
- ✅ Build Tests
- ✅ Integration & E2E Tests

### Optional Tests
These tests provide additional validation but don't block deployment:
- Docker Tests (informational)
- Security Scan (advisory)

## Usage Examples

### Local Testing
```bash
# Run the same tests as CI
make test-all

# Quick development feedback
make test-quick

# Check test status
make test-status
```

### Debugging Failed Pipelines
1. **Check Test Summary**: Download the test summary artifact
2. **Review Specific Job**: Check individual job logs
3. **Reproduce Locally**: Use the same commands from the pipeline
4. **Check Artifacts**: Download test results and coverage reports

## Performance Metrics

### Total Pipeline Duration
- **Parallel Execution**: ~8-12 minutes
- **Sequential Execution**: ~15-20 minutes

### Resource Usage
- **CPU**: 2 cores per job
- **Memory**: 4GB per job
- **Storage**: ~500MB artifacts per run

## Maintenance

### Adding New Tests
1. Add test files to appropriate `tests/` directory
2. Update `make test-all` if needed
3. Pipeline will automatically detect and run new tests

### Updating Dependencies
1. Update `go.mod` and `go.sum`
2. Update Docker base images if needed
3. Test locally with `make test-all`

### Security Updates
1. Monitor security scan results
2. Update dependencies regularly
3. Review and address security findings

## Troubleshooting

### Common Issues
1. **Database Connection Failures**: Check PostgreSQL service health
2. **Test Timeouts**: Increase wait times in workflow
3. **Build Failures**: Verify Go version compatibility
4. **Contract Test Failures**: Check OpenAPI spec alignment

### Support
- Check pipeline logs in GitHub Actions
- Review test artifacts for detailed error information
- Use `make test-status` to verify local test environment
