.PHONY: build run test tidy docker sqlc web mcp hearth-mcp

VERSION ?= dev
LDFLAGS := -s -w -X hearth/internal/buildinfo.Version=$(VERSION)

web:
	cd web && npm ci && npm run build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/hearth ./cmd/hearth

run: build
	./bin/hearth

test:
	go test ./...

tidy:
	go mod tidy

sqlc:
	sqlc generate

docker:
	docker build -t hearth:dev .

mcp hearth-mcp:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/hearth-mcp ./cmd/hearth-mcp
