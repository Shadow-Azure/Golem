#!/bin/bash
set -e

# E2E tests for Golem CLI commands
# This script tests the CLI commands end-to-end

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Build binary
echo -e "${YELLOW}Building Golem binary...${NC}"
CGO_ENABLED=0 go build -o /tmp/golem-e2e ./cmd/golem
GOLEM="/tmp/golem-e2e"

# Track failures
FAILURES=0

# Helper function to run test
run_test() {
    local test_name="$1"
    local expected_exit="$2"
    shift 2
    local cmd="$@"

    echo -e "${YELLOW}Test: $test_name${NC}"

    # Run command and capture exit code
    set +e
    OUTPUT=$($cmd 2>&1)
    ACTUAL_EXIT=$?
    set -e

    # Check exit code
    if [ "$ACTUAL_EXIT" -ne "$expected_exit" ]; then
        echo -e "${RED}  ❌ Expected exit code $expected_exit, got $ACTUAL_EXIT${NC}"
        echo "  Output: $OUTPUT"
        FAILURES=$((FAILURES + 1))
        return 1
    fi

    echo -e "${GREEN}  ✅ Passed${NC}"
    return 0
}

# Test 1: Version command
echo ""
echo -e "${YELLOW}=== Test Suite: Version Command ===${NC}"

run_test "version command" 0 $GOLEM version
if echo "$OUTPUT" | grep -q "Golem AI Agent"; then
    echo -e "${GREEN}  ✅ Version output contains 'Golem AI Agent'${NC}"
else
    echo -e "${RED}  ❌ Version output missing 'Golem AI Agent'${NC}"
    FAILURES=$((FAILURES + 1))
fi

# Test 2: Help command
echo ""
echo -e "${YELLOW}=== Test Suite: Help Command ===${NC}"

run_test "help command" 0 $GOLEM --help
if echo "$OUTPUT" | grep -q "start"; then
    echo -e "${GREEN}  ✅ Help output contains 'start' command${NC}"
else
    echo -e "${RED}  ❌ Help output missing 'start' command${NC}"
    FAILURES=$((FAILURES + 1))
fi

if echo "$OUTPUT" | grep -q "stop"; then
    echo -e "${GREEN}  ✅ Help output contains 'stop' command${NC}"
else
    echo -e "${RED}  ❌ Help output missing 'stop' command${NC}"
    FAILURES=$((FAILURES + 1))
fi

if echo "$OUTPUT" | grep -q "onboard"; then
    echo -e "${GREEN}  ✅ Help output contains 'onboard' command${NC}"
else
    echo -e "${RED}  ❌ Help output missing 'onboard' command${NC}"
    FAILURES=$((FAILURES + 1))
fi

# Test 3: Status command (when not running)
echo ""
echo -e "${YELLOW}=== Test Suite: Status Command ===${NC}"

run_test "status command (not running)" 0 $GOLEM status
if echo "$OUTPUT" | sed 's/\x1b\[[0-9;]*m//g' | grep -q "未运行"; then
    echo -e "${GREEN}  ✅ Status shows 'not running'${NC}"
else
    echo -e "${RED}  ❌ Status should show 'not running'${NC}"
    FAILURES=$((FAILURES + 1))
fi

# Test 4: Stop command (when not running)
echo ""
echo -e "${YELLOW}=== Test Suite: Stop Command ===${NC}"

run_test "stop command (not running)" 0 $GOLEM stop
if echo "$OUTPUT" | sed 's/\x1b\[[0-9;]*m//g' | grep -q "未运行"; then
    echo -e "${GREEN}  ✅ Stop shows 'not running'${NC}"
else
    echo -e "${RED}  ❌ Stop should show 'not running'${NC}"
    FAILURES=$((FAILURES + 1))
fi

# Test 5: Invalid command
echo ""
echo -e "${YELLOW}=== Test Suite: Invalid Command ===${NC}"

run_test "invalid command" 1 $GOLEM invalid-command

# Test 6: Config file flag
echo ""
echo -e "${YELLOW}=== Test Suite: Config Flag ===${NC}"

# Create temporary config
TEMP_CONFIG=$(mktemp /tmp/golem-test-XXXXXX.yaml)
cat > "$TEMP_CONFIG" << EOF
server:
  host: "127.0.0.1"
  port: 9999
llm:
  default_provider: "openai"
  providers:
    openai:
      api_key: "test-key"
      model: "gpt-4o"
session:
  max_history: 50
  trim_to: 20
logging:
  level: "info"
  format: "json"
EOF

# Test foreground with custom config (run in background and kill after 2 seconds)
echo -e "${YELLOW}Test: foreground with custom config${NC}"
$GOLEM --config "$TEMP_CONFIG" &
GOLEM_PID=$!
sleep 2
if kill -0 $GOLEM_PID 2>/dev/null; then
    echo -e "${GREEN}  ✅ Foreground process started successfully${NC}"
    kill $GOLEM_PID 2>/dev/null || true
else
    echo -e "${RED}  ❌ Foreground process failed to start${NC}"
    FAILURES=$((FAILURES + 1))
fi

# Cleanup
rm -f "$TEMP_CONFIG"

# Test 7: Build binary verification
echo ""
echo -e "${YELLOW}=== Test Suite: Binary Verification ===${NC}"

# Check binary size
BINARY_SIZE=$(du -h /tmp/golem-e2e | cut -f1)
echo -e "${GREEN}  Binary size: $BINARY_SIZE${NC}"

# Check binary is executable
if [ -x /tmp/golem-e2e ]; then
    echo -e "${GREEN}  ✅ Binary is executable${NC}"
else
    echo -e "${RED}  ❌ Binary is not executable${NC}"
    FAILURES=$((FAILURES + 1))
fi

# Cleanup
rm -f /tmp/golem-e2e

# Summary
echo ""
echo -e "${GREEN}============================================${NC}"
if [ $FAILURES -eq 0 ]; then
    echo -e "${GREEN}  ✅ All E2E tests passed!${NC}"
    echo -e "${GREEN}============================================${NC}"
    exit 0
else
    echo -e "${RED}  ❌ $FAILURES E2E test(s) failed${NC}"
    echo -e "${GREEN}============================================${NC}"
    exit 1
fi
