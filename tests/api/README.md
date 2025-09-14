# OpenAPI Contract Tests

This directory contains comprehensive tests to verify that the OpenAPI specification (`docs/api/openapi.yaml`) is fully aligned with the actual code implementation.

## Overview

The OpenAPI contract tests ensure that:

1. **API Endpoints**: All endpoints defined in the OpenAPI spec exist in the implementation
2. **HTTP Methods**: Correct HTTP methods are supported for each endpoint
3. **Request/Response Structures**: JSON schemas match between spec and Go structs
4. **Authentication**: Security requirements are properly implemented
5. **Schema Validation**: OpenAPI schema definitions match actual data structures

## Test Files

### `openapi_contract_test.go`
- **Endpoint Existence**: Verifies all OpenAPI endpoints exist in the HTTP handler
- **Request/Response Validation**: Tests actual API calls and validates response structures
- **Authentication Testing**: Ensures auth requirements match between spec and implementation
- **HTTP Method Validation**: Confirms only specified methods are accepted

### `schema_validation_test.go`
- **Schema Alignment**: Validates Go structs against OpenAPI schema definitions
- **Field Validation**: Ensures all required fields are present and correctly typed
- **JSON Serialization**: Tests that Go structs serialize to match OpenAPI examples

## Running the Tests

### Quick Start
```bash
# Run all OpenAPI contract tests
make test-openapi-contract

# Or run directly
./scripts/test-openapi-contract.sh
```

### Individual Test Suites
```bash
# Run from the tests/api directory
cd tests/api

# Run contract alignment tests
go test -v -run TestOpenAPIContractAlignment

# Run schema validation tests
go test -v -run TestSchemaValidation

# Run all tests
go test -v ./...
```

### Integration with CI/CD
The OpenAPI contract tests are automatically included in:
- `make test-all` - Runs all test suites including OpenAPI contract tests
- GitHub Actions CI/CD pipeline
- Pre-commit hooks (if configured)

## Test Coverage

### API Endpoints Tested
- ✅ `GET /health` - Health check (no auth required)
- ✅ `POST /place-order` - Place trading orders
- ✅ `POST /cancel-order` - Cancel existing orders
- ✅ `GET /positions` - Get current positions
- ✅ `POST /close-position` - Close positions
- ✅ `GET /orders/{orderId}` - Get order status
- ✅ `GET /order-history` - Get order history with filtering

### Schema Definitions Tested
- ✅ `OrderRequest` - Order placement request structure
- ✅ `OrderResponse` - Order operation response
- ✅ `CancelOrderRequest` - Order cancellation request
- ✅ `CancelOrderResponse` - Order cancellation response
- ✅ `PositionsResponse` - Positions query response
- ✅ `ClosePositionRequest` - Position closing request
- ✅ `ClosePositionResponse` - Position closing response
- ✅ `OrderStatusResponse` - Order status query response
- ✅ `OrderHistoryResponse` - Order history query response
- ✅ `HealthResponse` - Health check response
- ✅ `ErrorResponse` - Error response structure

### Authentication Testing
- ✅ API Key authentication via `X-API-Key` header
- ✅ Bearer token authentication via `Authorization` header
- ✅ Health endpoint bypass (no auth required)
- ✅ 401 Unauthorized responses for missing/invalid credentials

### HTTP Method Validation
- ✅ GET methods only accept GET requests
- ✅ POST methods only accept POST requests
- ✅ Unsupported methods return 405 Method Not Allowed
- ✅ Content-Type validation for POST requests

## Test Architecture

### Mock Service
The tests use a `MockService` that implements the `service.Service` interface with realistic test data. This allows testing the HTTP transport layer without requiring actual business logic or external dependencies.

### OpenAPI Validation
Tests use the `github.com/getkin/kin-openapi` library to:
- Load and validate the OpenAPI specification
- Perform schema validation against actual JSON data
- Verify endpoint definitions and operations

### HTTP Testing
Tests create an actual HTTP server using `httptest.NewServer` to:
- Test real HTTP requests and responses
- Validate middleware behavior (auth, CORS, etc.)
- Ensure proper content-type handling

## Troubleshooting

### Common Issues

1. **OpenAPI Spec Not Found**
   ```
   ❌ OpenAPI specification not found at: docs/api/openapi.yaml
   ```
   - Ensure the OpenAPI spec exists at the correct path
   - Run tests from the project root directory

2. **Schema Validation Failures**
   ```
   Schema validation failed: field 'orderId' is required but missing
   ```
   - Check that Go struct JSON tags match OpenAPI field names
   - Ensure required fields are marked correctly in both places

3. **Endpoint Not Found**
   ```
   Endpoint POST /place-order should exist in implementation
   ```
   - Verify the HTTP handler registers the correct routes
   - Check that endpoint paths match exactly between spec and code

4. **Authentication Failures**
   ```
   Endpoint should require authentication but returned 200
   ```
   - Ensure middleware is properly configured
   - Check that auth bypass logic only applies to intended endpoints

### Debugging Tips

1. **Enable Verbose Output**
   ```bash
   go test -v -run TestOpenAPIContractAlignment
   ```

2. **Run Individual Test Cases**
   ```bash
   go test -v -run "TestOpenAPIContractAlignment/EndpointExistence"
   ```

3. **Check OpenAPI Spec Validity**
   ```bash
   # Use online validators or tools like swagger-codegen
   swagger-codegen validate -i docs/api/openapi.yaml
   ```

## Maintenance

### Adding New Endpoints
When adding new API endpoints:

1. **Update OpenAPI Spec**: Add endpoint definition to `docs/api/openapi.yaml`
2. **Update Contract Tests**: Add endpoint to test cases in `openapi_contract_test.go`
3. **Update Schema Tests**: Add schema validation in `schema_validation_test.go`
4. **Update Mock Service**: Add mock implementation for new service methods

### Updating Existing Endpoints
When modifying existing endpoints:

1. **Update OpenAPI Spec**: Modify schema definitions and endpoint details
2. **Update Go Structs**: Ensure struct fields match schema changes
3. **Run Tests**: Verify all contract tests still pass
4. **Update Test Data**: Modify mock responses if needed

The OpenAPI contract tests provide confidence that your API documentation stays in sync with your implementation, preventing the common problem of outdated API documentation.
