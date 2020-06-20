# Go parameters
GO=go
GOBUILD=$(GO) build
GOCLEAN=$(GO) clean
GOTEST=$(GO) test
GOGET=$(GO) get
BUILDDIR=bin
NAME=docker-recreate
VERSION=$(shell cat VERSION.txt)

.PHONY: all
all: test build

.PHONY: build
build:
	$(GOBUILD) -o bin/$(NAME) -v

.PHONY: test
test:
	$(GOTEST) -v ./...

.PHONY: clean
clean:
	$(GOCLEAN)
	rm -f bin/*

.PHONY: run
run:
	$(GOBUILD) -o $(BUILDDIR)/$(NAME) -v ./...
	./$(BUILDDIR)/$(NAME)

.PHONY: deps
deps:
	$(GOGET) -v -t -d ./...

define buildrelease
GOOS=$(1) GOARCH=$(2) CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) \
	 -o $(BUILDDIR)/$(NAME)-$(1)-$(2) -a .;
endef

# List the GOOS and GOARCH to build
GOOSARCHES = $(shell cat .goosarch)

.PHONY: release
release: *.go VERSION.txt
	$(foreach GOOSARCH,$(GOOSARCHES), $(call buildrelease,$(subst /,,$(dir $(GOOSARCH))),$(notdir $(GOOSARCH))))
