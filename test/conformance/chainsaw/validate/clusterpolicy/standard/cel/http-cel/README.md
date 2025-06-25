## Description

This test validates the HTTP CEL library functionality in Kyverno ValidatingPolicy. It tests various HTTP operations including:
- GET requests with and without headers
- POST requests with and without headers
- Response status code and body validation

## Expected Behavior

The policy should be able to:
1. Make HTTP GET requests and process responses
2. Make HTTP POST requests and process responses
3. Send and validate HTTP headers
4. Handle various response status codes and body formats

## Related Issue

https://github.com/kyverno/kyverno/issues/12690 