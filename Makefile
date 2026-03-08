APP_NAME := nmd
WAILS_GOFLAGS := -ldflags=-linkmode=external

.PHONY: fmt dev build test

fmt:
	go fmt ./...

dev:
	GOFLAGS='$(WAILS_GOFLAGS)' wails dev

build:
	GOFLAGS='$(WAILS_GOFLAGS)' wails build

test:
	go test ./...
