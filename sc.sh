go test -coverprofile=coverage.out ./pkg/cel/policies/gpol/compiler

go tool cover -html=coverage.out