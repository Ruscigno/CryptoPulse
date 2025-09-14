#!/bin/bash

# OpenAPI Contract Testing Script
# This script runs comprehensive tests to verify that the OpenAPI specification
# is aligned with the actual code implementation.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
API_TEST_DIR="$PROJECT_ROOT/tests/api"

echo -e "${BLUE}🔍 OpenAPI Contract Testing${NC}"
echo "=================================================="

# Check if OpenAPI spec exists
OPENAPI_SPEC="$PROJECT_ROOT/docs/api/openapi.yaml"
if [[ ! -f "$OPENAPI_SPEC" ]]; then
    echo -e "${RED}❌ OpenAPI specification not found at: $OPENAPI_SPEC${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Found OpenAPI specification${NC}"

# Check if test directory exists
if [[ ! -d "$API_TEST_DIR" ]]; then
    echo -e "${RED}❌ API test directory not found at: $API_TEST_DIR${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Found API test directory${NC}"

# Change to test directory
cd "$API_TEST_DIR"

# Initialize Go module if go.sum doesn't exist
if [[ ! -f "go.sum" ]]; then
    echo -e "${YELLOW}📦 Initializing Go module dependencies...${NC}"
    go mod tidy
fi

# Run the contract tests
echo -e "${BLUE}🧪 Running OpenAPI Contract Tests...${NC}"
echo "--------------------------------------------------"

# Test 1: OpenAPI Contract Alignment
echo -e "${YELLOW}Test 1: API Contract Alignment${NC}"
if go test -v -run TestOpenAPIContractAlignment ./...; then
    echo -e "${GREEN}✅ API Contract Alignment: PASSED${NC}"
else
    echo -e "${RED}❌ API Contract Alignment: FAILED${NC}"
    exit 1
fi

echo ""

# Test 2: Schema Validation
echo -e "${YELLOW}Test 2: Schema Validation${NC}"
if go test -v -run TestSchemaValidation ./...; then
    echo -e "${GREEN}✅ Schema Validation: PASSED${NC}"
else
    echo -e "${RED}❌ Schema Validation: FAILED${NC}"
    exit 1
fi

echo ""

# Run all API tests
echo -e "${YELLOW}Running All API Tests${NC}"
if go test -v ./...; then
    echo -e "${GREEN}✅ All API Tests: PASSED${NC}"
else
    echo -e "${RED}❌ Some API Tests: FAILED${NC}"
    exit 1
fi

echo ""
echo "=================================================="
echo -e "${GREEN}🎉 All OpenAPI Contract Tests Passed!${NC}"
echo ""
echo -e "${BLUE}Summary:${NC}"
echo "• ✅ OpenAPI specification is valid"
echo "• ✅ All API endpoints exist in both spec and implementation"
echo "• ✅ Request/response structures match between spec and code"
echo "• ✅ Authentication requirements are correctly implemented"
echo "• ✅ HTTP methods are properly configured"
echo "• ✅ Schema definitions match Go struct definitions"
echo ""
echo -e "${GREEN}The OpenAPI specification is fully aligned with the code implementation!${NC}"
