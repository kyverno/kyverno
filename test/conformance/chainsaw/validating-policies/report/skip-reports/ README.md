cat > README.md << 'EOF'
# Skip Reports Test

This test verifies that the `kyverno.io/skip-reports: "true"` annotation works correctly.

## Test Scenario

1. **Policy with skip annotation**: `test-policy-with-skip` has `kyverno.io/skip-reports: "true"`
2. **Policy without skip annotation**: `test-policy-without-skip` has no skip annotation
3. **Test resource**: A Deployment that violates both policies (missing app label)

## Expected Results

-  Policy with skip annotation should generate **0 reports**
-  Policy without skip annotation should generate **1+ reports**

## Files

- `chainsaw-test.yaml` - Main test definition
- `policy-with-skip.yaml` - Policy with skip-reports annotation
- `policy-without-skip.yaml` - Policy without skip annotation
- `deployment.yaml` - Test resource that violates policies
- `README.md` - This documentation

## Running the Test

