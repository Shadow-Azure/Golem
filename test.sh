#!/bin/bash
set -e

echo "============================================"
echo "  Golem Local Quality Gate"
echo "============================================"
echo ""

FAILED=0

# 1. Rust format check
echo "[1/5] Rust format check..."
if cargo fmt --all -- --check; then
  echo "  ✅ cargo fmt passed"
else
  echo "  ❌ cargo fmt failed"
  FAILED=1
fi
echo ""

# 2. Rust Clippy
echo "[2/5] Rust Clippy..."
if cargo clippy -p golem-core -p golem-cli -p golem-daemon --all-targets -- -D warnings; then
  echo "  ✅ clippy passed"
else
  echo "  ❌ clippy failed"
  FAILED=1
fi
echo ""

# 3. Rust unit tests
echo "[3/5] Rust unit tests..."
if cargo test -p golem-core; then
  echo "  ✅ cargo test passed"
else
  echo "  ❌ cargo test failed"
  FAILED=1
fi
echo ""

# 4. Frontend type check
echo "[4/5] Frontend type check..."
if command -v pnpm &>/dev/null && [ -f electron-app/package.json ]; then
  cd electron-app
  if pnpm install --frozen-lockfile 2>/dev/null && pnpm typecheck; then
    echo "  ✅ typecheck passed"
  else
    echo "  ❌ typecheck failed"
    FAILED=1
  fi
  cd ..
else
  echo "  ⏭️  skipped (pnpm not installed or no electron-app)"
fi
echo ""

# 5. Frontend unit tests
echo "[5/5] Frontend unit tests..."
if command -v pnpm &>/dev/null && [ -f electron-app/package.json ]; then
  cd electron-app
  if pnpm vitest run 2>/dev/null; then
    echo "  ✅ vitest passed"
  else
    echo "  ❌ vitest failed"
    FAILED=1
  fi
  cd ..
else
  echo "  ⏭️  skipped (pnpm not installed or no electron-app)"
fi
echo ""

echo "============================================"
if [ "$FAILED" -eq 0 ]; then
  echo "  ✅ All checks passed!"
  exit 0
else
  echo "  ❌ Some checks failed"
  exit 1
fi
