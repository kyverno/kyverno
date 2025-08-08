go test -coverprofile=coverage.out ./pkg/background/gpol

go tool cover -html=coverage.out