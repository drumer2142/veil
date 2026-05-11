.PHONY: help tidy build run clean fmt vet test docker-build docker-up docker-down docker-logs

BIN      ?= dashboard
ADDR     ?= :8080
DATA     ?= ./data
IMAGE    ?= overseer-dashboard
GO       ?= go

help:
	@echo "Targets:"
	@echo "  make tidy         - go mod tidy"
	@echo "  make build        - build $(BIN) locally (CGO_ENABLED=0)"
	@echo "  make run          - run locally (ADDR=$(ADDR) DATA=$(DATA))"
	@echo "  make fmt          - go fmt ./..."
	@echo "  make vet          - go vet ./..."
	@echo "  make test         - go test ./..."
	@echo "  make clean        - remove $(BIN)"
	@echo "  make docker-build - docker build -t $(IMAGE)"
	@echo "  make docker-up    - docker compose up --build -d"
	@echo "  make docker-down  - docker compose down"
	@echo "  make docker-logs  - docker compose logs -f"

tidy:
	$(GO) mod tidy

build: tidy
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN) .

run: build
	./$(BIN) -addr $(ADDR) -data $(DATA)

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

clean:
	rm -f $(BIN)

build:
	docker build -t $(IMAGE) .

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f
