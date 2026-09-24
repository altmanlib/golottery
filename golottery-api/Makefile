.DEFAULT_GOAL := test

APP_NAME := golottery
VERSION := $(shell cat VERSION 2>/dev/null || echo dev)
BUILD_DIR := build
BINARY := $(BUILD_DIR)/$(APP_NAME)

.PHONY: all build run dev test fmt lint tidy generate clean help

all: build

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BINARY) ./cmd/golottery

run:
	go run ./cmd/golottery

dev:
	@set -e; set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/golottery

test:
	go test -p=1 ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

generate:
	go generate ./...

clean:
	rm -rf $(BUILD_DIR)

help:
	@echo "Available targets:"
	@echo "  make build      Build binary to $(BINARY)"
	@echo "  make run        Run the service"
	@echo "  make dev        Run with .env loaded"
	@echo "  make test       Run tests against golottery_test"
	@echo "  make fmt        Format Go sources"
	@echo "  make lint       Run golangci-lint"
	@echo "  make generate   Regenerate OpenAPI server code"
	@echo "  make tidy       Tidy go.mod"
