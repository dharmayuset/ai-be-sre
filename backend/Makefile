.PHONY: help tidy fmt vet test build run clean docker

help:
	@echo "Available targets:"
	@echo "  tidy   - go mod tidy"
	@echo "  fmt    - gofmt -w ."
	@echo "  vet    - go vet ./..."
	@echo "  test   - go test ./..."
	@echo "  build  - build binary ke ./bin/server"
	@echo "  run    - run server lokal"
	@echo "  clean  - hapus binary"
	@echo "  docker - build docker image"

tidy:
	go mod tidy

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test -count=1 ./...

build:
	mkdir -p bin
	CGO_ENABLED=1 go build -ldflags="-s -w" -o bin/server ./cmd/server

run:
	go run ./cmd/server

clean:
	rm -rf bin

docker:
	docker build -t ai-be-sre-backend:dev .
