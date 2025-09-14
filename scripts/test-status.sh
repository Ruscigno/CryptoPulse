#!/bin/bash

# Test Status Script
# Shows available tests and their current status

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo -e "${BLUE}🧪 CryptoPulse Test Suite Status${NC}"
echo "=================================================="

# Function to check if directory has Go test files
has_tests() {
    local dir="$1"
    if [[ -d "$dir" ]] && find "$dir" -name "*_test.go" -type f | grep -q .; then
        return 0
    else
        return 1
    fi
}

# Function to count test files
count_tests() {
    local dir="$1"
    if [[ -d "$dir" ]]; then
        find "$dir" -name "*_test.go" -type f | wc -l | tr -d ' '
    else
        echo "0"
    fi
}

# Function to get test status
get_status() {
    local has_tests="$1"
    if [[ "$has_tests" == "true" ]]; then
        echo -e "${GREEN}✅ Available${NC}"
    else
        echo -e "${YELLOW}⚠️  Not Found${NC}"
    fi
}

echo -e "${CYAN}Test Categories:${NC}"
echo ""

# Unit Tests
unit_count=$(count_tests "$PROJECT_ROOT/tests/unit")
if has_tests "$PROJECT_ROOT/tests/unit"; then
    unit_status="${GREEN}✅ Available${NC}"
else
    unit_status="${YELLOW}⚠️  Not Found${NC}"
fi

echo -e "📋 ${BLUE}Unit Tests${NC}"
echo -e "   Location: tests/unit/"
echo -e "   Files: $unit_count test files"
echo -e "   Status: $unit_status"
echo -e "   Command: ${CYAN}make test-unit${NC}"
echo ""

# Integration Tests
integration_count=$(count_tests "$PROJECT_ROOT/tests/integration")
if has_tests "$PROJECT_ROOT/tests/integration"; then
    integration_status="${GREEN}✅ Available${NC}"
else
    integration_status="${YELLOW}⚠️  Not Found${NC}"
fi

echo -e "🔗 ${BLUE}Integration Tests${NC}"
echo -e "   Location: tests/integration/"
echo -e "   Files: $integration_count test files"
echo -e "   Status: $integration_status"
echo -e "   Command: ${CYAN}make test-integration${NC}"
echo -e "   Note: Requires database setup"
echo ""

# OpenAPI Contract Tests
contract_count=$(count_tests "$PROJECT_ROOT/tests/api")
if has_tests "$PROJECT_ROOT/tests/api"; then
    contract_status="${GREEN}✅ Available${NC}"
else
    contract_status="${YELLOW}⚠️  Not Found${NC}"
fi

echo -e "📄 ${BLUE}OpenAPI Contract Tests${NC}"
echo -e "   Location: tests/api/"
echo -e "   Files: $contract_count test files"
echo -e "   Status: $contract_status"
echo -e "   Command: ${CYAN}make test-openapi-contract${NC}"
echo ""

# E2E Tests
e2e_count=$(count_tests "$PROJECT_ROOT/tests/e2e")
if has_tests "$PROJECT_ROOT/tests/e2e"; then
    e2e_status="${GREEN}✅ Available${NC}"
else
    e2e_status="${YELLOW}⚠️  Not Found${NC}"
fi

echo -e "🌐 ${BLUE}End-to-End Tests${NC}"
echo -e "   Location: tests/e2e/"
echo -e "   Files: $e2e_count test files"
echo -e "   Status: $e2e_status"
echo -e "   Command: ${CYAN}make test-e2e${NC}"
echo -e "   Note: Requires full application setup"
echo ""

echo "=================================================="
echo -e "${CYAN}Available Test Commands:${NC}"
echo ""
echo -e "• ${CYAN}make test${NC}           - Quick tests (unit + contract)"
echo -e "• ${CYAN}make test-quick${NC}     - Same as 'make test'"
echo -e "• ${CYAN}make test-unit${NC}      - Unit tests only"
echo -e "• ${CYAN}make test-integration${NC} - Integration tests with DB setup"
echo -e "• ${CYAN}make test-openapi-contract${NC} - OpenAPI contract validation"
echo -e "• ${CYAN}make test-all${NC}       - All tests (unit + integration + contract + e2e)"
echo -e "• ${CYAN}make test-ci${NC}        - CI/CD test suite with full setup"
echo -e "• ${CYAN}make test-coverage${NC}  - Tests with coverage report"
echo ""

# Check dependencies
echo "=================================================="
echo -e "${CYAN}Dependencies Status:${NC}"
echo ""

# Check Docker
if command -v docker >/dev/null 2>&1; then
    echo -e "🐳 Docker: ${GREEN}✅ Available${NC}"
else
    echo -e "🐳 Docker: ${RED}❌ Not Found${NC}"
fi

# Check Docker Compose
if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    echo -e "🐙 Docker Compose: ${GREEN}✅ Available${NC}"
else
    echo -e "🐙 Docker Compose: ${RED}❌ Not Found${NC}"
fi

# Check Go
if command -v go >/dev/null 2>&1; then
    go_version=$(go version | cut -d' ' -f3)
    echo -e "🐹 Go: ${GREEN}✅ Available${NC} ($go_version)"
else
    echo -e "🐹 Go: ${RED}❌ Not Found${NC}"
fi

# Check migrate tool
if command -v migrate >/dev/null 2>&1; then
    echo -e "📊 golang-migrate: ${GREEN}✅ Available${NC}"
else
    echo -e "📊 golang-migrate: ${YELLOW}⚠️  Will be installed when needed${NC}"
fi

echo ""
echo "=================================================="

# Calculate total tests
total_tests=$((unit_count + integration_count + contract_count + e2e_count))
echo -e "${BLUE}Summary:${NC}"
echo -e "• Total test files: $total_tests"
echo -e "• Unit: $unit_count files"
echo -e "• Integration: $integration_count files"
echo -e "• Contract: $contract_count files"
echo -e "• E2E: $e2e_count files"
echo ""

if [[ $total_tests -gt 0 ]]; then
    echo -e "${GREEN}🎉 Test suite is ready!${NC}"
    echo -e "Run ${CYAN}make test${NC} for quick tests or ${CYAN}make test-all${NC} for comprehensive testing."
else
    echo -e "${RED}⚠️  No tests found!${NC}"
    echo "Consider adding tests to ensure code quality."
fi
