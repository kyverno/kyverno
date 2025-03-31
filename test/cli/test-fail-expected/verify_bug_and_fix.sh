#!/bin/bash

set -e

echo "=== STEP 1: Building Kyverno CLI with buggy code ==="
cd ../../..
go build -o kyverno-buggy ./cmd/cli/kubectl-kyverno

echo -e "\n=== STEP 2: Running test case with buggy code ==="
cd test/cli/test-fail-expected
../../../kyverno-buggy test .
echo -e "\nWith the bug, the test shows as PASS when it should FAIL"

echo -e "\n=== STEP 3: Restoring fixed code ==="
cd ../../..
mv cmd/cli/kubectl-kyverno/commands/test/output.go.fixed cmd/cli/kubectl-kyverno/commands/test/output.go
go build -o kyverno-fixed ./cmd/cli/kubectl-kyverno

echo -e "\n=== STEP 4: Running test case with fixed code ==="
cd test/cli/test-fail-expected
../../../kyverno-fixed test .
echo -e "\nAfter the fix, the test correctly shows as FAIL" 