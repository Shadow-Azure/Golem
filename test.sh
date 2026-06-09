#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  Golem Quality Gate${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

# Track failures
FAILURES=0

# Step 1: Go format check
echo -e "${YELLOW}[1/5] Go Format Check${NC}"
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    echo -e "${RED}  ❌ The following files are not formatted:${NC}"
    echo "$UNFORMATTED"
    echo ""
    echo "  Run 'gofmt -s -w .' to fix"
    FAILURES=$((FAILURES + 1))
else
    echo -e "${GREEN}  ✅ All Go files are properly formatted${NC}"
fi
echo ""

# Step 2: Go vet
echo -e "${YELLOW}[2/5] Go Vet${NC}"
if go vet ./...; then
    echo -e "${GREEN}  ✅ Go vet passed${NC}"
else
    echo -e "${RED}  ❌ Go vet failed${NC}"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# Step 3: Go tests
echo -e "${YELLOW}[3/5] Go Tests${NC}"
if go test -v -race ./...; then
    echo -e "${GREEN}  ✅ All Go tests passed${NC}"
else
    echo -e "${RED}  ❌ Go tests failed${NC}"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# Step 4: Go build
echo -e "${YELLOW}[4/5] Go Build${NC}"
if CGO_ENABLED=0 go build -o /tmp/golem-test ./cmd/golem; then
    echo -e "${GREEN}  ✅ Go build succeeded${NC}"
    rm -f /tmp/golem-test
else
    echo -e "${RED}  ❌ Go build failed${NC}"
    FAILURES=$((FAILURES + 1))
fi
echo ""

# Step 5: E2E tests
echo -e "${YELLOW}[5/5] E2E Tests${NC}"
if [ -f "tests/e2e-cli.sh" ]; then
    if bash tests/e2e-cli.sh; then
        echo -e "${GREEN}  ✅ E2E tests passed${NC}"
    else
        echo -e "${RED}  ❌ E2E tests failed${NC}"
        FAILURES=$((FAILURES + 1))
    fi
else
    echo -e "${YELLOW}  ⏭️  E2E test script not found, skipping${NC}"
fi
echo ""

# Summary
echo -e "${GREEN}============================================${NC}"
if [ $FAILURES -eq 0 ]; then
    echo -e "${GREEN}  ✅ All quality checks passed!${NC}"
    echo -e "${GREEN}============================================${NC}"
    exit 0
else
    echo -e "${RED}  ❌ $FAILURES quality check(s) failed${NC}"
    echo -e "${GREEN}============================================${NC}"
    exit 1
fi
