.PHONY: build deps
SHELL=/bin/bash
export GOPRIVATE=github.com/anyproto
export PATH:=deps:$(PATH)
export CGO_ENABLED:=1
BUILD_GOOS:=$(shell go env GOOS)
BUILD_GOARCH:=$(shell go env GOARCH)

ifeq ($(CGO_ENABLED), 0)
	TAGS:=-tags nographviz
else
	TAGS:=
endif
ifeq ($(BUILD_GOOS), windows)
	BIN_SUFFUX:=.exe
else
	BIN_SUFFUX:=
endif

build:
	GOOS=$(BUILD_GOOS) GOARCH=$(BUILD_GOARCH) go build -v $(TAGS) -o bin/any-sync-network$(BIN_SUFFUX) ./any-sync-network
	GOOS=$(BUILD_GOOS) GOARCH=$(BUILD_GOARCH) go build -v $(TAGS) -o bin/any-sync-netcheck$(BIN_SUFFUX) ./any-sync-netcheck

deps:
	go mod download
