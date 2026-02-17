#!/bin/bash -e
# Run unit tests with coverage
go test -v -count=1 -coverprofile=coverage.out ./... 2>&1 | tee test-output.txt

# Print test summary
echo ""
echo "=== TEST SUMMARY ==="
PASS_COUNT=$(grep -c "^--- PASS:" test-output.txt || true)
FAIL_COUNT=$(grep -c "^--- FAIL:" test-output.txt || true)
SKIP_COUNT=$(grep -c "^--- SKIP:" test-output.txt || true)
TOTAL=$((PASS_COUNT + FAIL_COUNT + SKIP_COUNT))
echo "Total: $TOTAL | Passed: $PASS_COUNT | Failed: $FAIL_COUNT | Skipped: $SKIP_COUNT"

# Print coverage
echo ""
echo "=== CODE COVERAGE ==="
go tool cover -func=coverage.out | tail -1

# Cleanup
rm -f test-output.txt coverage.out
