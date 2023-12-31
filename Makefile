GOTESTSUM_FORMAT ?= pkgname-and-test-fails
.PHONY: test
test: lint
	@echo "starting gotestsum..."
	@go run gotest.tools/gotestsum@v1.11.0 -f $(GOTESTSUM_FORMAT) -- -coverprofile=coverage.out  ./...
.PHONY: test-verbose
test-verbose:
	GOTESTSUM_FORMAT=standard-verbose $(MAKE) test

.PHONY: lint
lint:
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2 run \
	 -E goimports \
	 -E misspell \
	 -E lll \
	 -E gocritic \
	 -E goimports ./...

.PHONY: fmt
fmt: 
	@echo "go fmt"
	@gofmt -s -w . 
	@echo "go fmt done"

.PHONY: dependencies
dependencies:
	@echo "installing dependencies"
	@asdf plugin add golang
	@asdf plugin add yq
	@asdf install

.PHONY: build
build:
	@echo "building..."
	@go build -o bin/ ./...