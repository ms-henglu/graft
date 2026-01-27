.PHONY: fmt test lint

fmt:
	go fmt ./...

test:
	go test -v ./...

lint:
	golangci-lint run
