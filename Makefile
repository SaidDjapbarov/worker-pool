.PHONY: run test lint

run:
	@go run ./cmd/demo

test:
	@go test -race ./...

lint:
	@golangci-lint run
