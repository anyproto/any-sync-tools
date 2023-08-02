.PHONY: build deps
SHELL=/bin/bash
export PATH:=deps:$(PATH)
BUILD_GOOS:=$(shell go env GOOS)
BUILD_GOARCH:=$(shell go env GOARCH)

ifeq ($(BUILD_GOOS), windows)
	BIN_SUFFUX:=.exe
else
	BIN_SUFFUX:=
endif

build:
	@$(eval FLAGS := $$(shell PATH=$(PATH) govvv -flags -pkg github.com/anyproto/any-sync/app))
	GOOS=$(BUILD_GOOS) GOARCH=$(BUILD_GOARCH) go build -ldflags "$(FLAGS)" -o bin/any-sync-network$(BIN_SUFFUX) ./any-sync-network
	GOOS=$(BUILD_GOOS) GOARCH=$(BUILD_GOARCH) go build -ldflags "$(FLAGS)" -o bin/any-sync-netcheck$(BIN_SUFFUX) ./any-sync-netcheck

deps:
	go mod download
