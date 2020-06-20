# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=docker-recreate

.PHONY: all
all: test build

.PHONY: build
build:
				$(GOBUILD) -o bin/$(BINARY_NAME) -v

.PHONY: test
test:
				$(GOTEST) -v ./...

.PHONY: clean
clean:
				$(GOCLEAN)
				rm -f bin/*

.PHONY: run
run:
				$(GOBUILD) -o $(BINARY_NAME) -v ./...
				./$(BINARY_NAME)

.PHONY: deps
deps:
				$(GOGET) -v -t -d ./...
