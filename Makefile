# Go parameters
GO=go
GOBUILD=$(GO) build
GOCLEAN=$(GO) clean
GOTEST=$(GO) test
BUILDDIR=bin
NAME=docker-recreate
VERSION=$(strip $(shell head -n 1 VERSION.txt))
GOOSARCHES=$(shell cat .goosarch)
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

.PHONY: all
all: deps test build

.PHONY: build
build:
	$(GOBUILD) $(LDFLAGS) -o bin/$(NAME) -v

.PHONY: install
install:
	cp bin/$(NAME) ~/.docker/cli-plugins/

.PHONY: test
test:
	$(GOTEST) -v ./...

.PHONY: clean
clean:
	$(GOCLEAN)
	rm -f bin/*

.PHONY: run
run:
	$(GOBUILD) $(LDFLAGS) -o $(BUILDDIR)/$(NAME) -v ./...
	./$(BUILDDIR)/$(NAME)

.PHONY: deps
deps:
	$(GO) mod download

define buildrelease
GOOS=$(1) GOARCH=$(2) CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) \
	 $(LDFLAGS) -o $(BUILDDIR)/$(NAME)-$(1)-$(2) -a .;
endef

.PHONY: release
release: *.go VERSION.txt
	$(foreach GOOSARCH,$(GOOSARCHES), $(call buildrelease,$(subst /,,$(dir $(GOOSARCH))),$(notdir $(GOOSARCH))))

.PHONY: tag
tag: ## Create a new git tag to prepare to build a release.
	git tag -a $(VERSION) -m "$(VERSION)"
	@echo "Run \"git push origin $(VERSION)\" to push your new tag to GitHub and trigger a release."
