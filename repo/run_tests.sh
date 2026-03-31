#!/bin/bash
set -e

echo "=== ParkOps Test Suite ==="
echo ""

if ! docker compose ps --status running 2>/dev/null | grep -q "app"; then
  echo "--- Containers not running, starting... ---"
  docker compose up -d
  echo "--- Waiting for DB to be healthy... ---"
  sleep 8
fi

echo "--- Running tests ---"
docker compose exec -T app sh -c "
  cd /app && \
  TEST_DATABASE_URL='postgres://parkops:parkops@127.0.0.1:5432/parkops?sslmode=disable' \
  go test -mod=mod ./unit_tests/... ./API_tests/... -v -count=1
"
EXIT=$?

if [ $EXIT -eq 0 ]; then
  echo "=== ALL TESTS PASSED ==="
else
  echo "=== TESTS FAILED ==="
fi

exit $EXIT
