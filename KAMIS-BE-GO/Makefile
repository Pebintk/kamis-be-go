SERVICE ?= template

.PHONY: tidy run build test vet fmt

tidy:
	go mod tidy

run:
	go run ./services/$(SERVICE)/cmd

build:
	CGO_ENABLED=0 go build -trimpath -o bin/$(SERVICE) ./services/$(SERVICE)/cmd

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .
