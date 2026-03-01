.PHONY: build
build:
	go build -v ./cmd/apiserver
	apiserver

.DEFAULT_GOAL := build
