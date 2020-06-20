# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=docker-recreate

all: test build
build:
				$(GOBUILD) -o bin/$(BINARY_NAME) -v
test:
				$(GOTEST) -v ./...
clean:
				$(GOCLEAN)
				rm -f bin/*
run:
				$(GOBUILD) -o $(BINARY_NAME) -v ./...
				./$(BINARY_NAME)
deps:
				$(GOGET) -v -t -d ./...
