## Description

This test validates the HTTP CEL library functionality in Kyverno policies. It tests various HTTP operations including:
- GET requests with and without headers
- POST requests with and without headers
- Client creation with custom CA bundle

## Expected Behavior

The policy should be able to:
1. Make HTTP GET requests and process responses
2. Make HTTP POST requests and process responses
3. Create HTTP clients with custom CA bundles
4. Handle various response status codes and body formats

## Related Issue

https://github.com/kyverno/kyverno/issues/12690 