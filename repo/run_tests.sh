#!/bin/bash
set -e

echo "=== ParkOps Test Suite ==="
echo ""

if ! docker compose ps --status running 2>/dev/null | grep -q "app"; then
  echo "--- Containers not running, starting... ---"
  docker compose up -d
  echo "--- Waiting for app to be ready... ---"
  for i in $(seq 1 30); do
    if docker compose exec -T app wget -qO- http://localhost:8080/api/health >/dev/null 2>&1; then
      echo "--- App is ready ---"
      break
    fi
    echo "  waiting... ($i/30)"
    sleep 3
  done
fi

echo "--- Live TCP smoke check against running app ---"
docker compose exec -T app sh -c "
  set -e
  wget -qO- http://localhost:8080/api/health | grep -q '\"status\":\"ok\"' || { echo 'health over TCP failed'; exit 1; }
  code=\$(wget -S -qO /dev/null http://localhost:8080/dashboard 2>&1 | awk '/HTTP\\//{print \$2; exit}')
  [ \"\$code\" = '303' ] || { echo \"dashboard expected 303 over TCP, got \$code\"; exit 1; }
  echo 'live TCP smoke: OK'
"

echo "--- Running tests ---"
docker compose exec -T app sh -c "
  cd /app && \
  TEST_DATABASE_URL='postgres://parkops:parkops@127.0.0.1:5432/parkops?sslmode=disable' \
  go test -mod=mod ./unit_tests/... ./API_tests/... ./internal/web/... -v -count=1
"
EXIT=$?

echo ""
if [ $EXIT -eq 0 ]; then
  echo "=== ALL TESTS PASSED ==="
else
  echo "=== TESTS FAILED ==="
fi

exit $EXIT
