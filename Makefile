BINARY_NAME := ksef
BUILD_DIR := bin
COMPLETIONS_DIR := $(BUILD_DIR)/completions
INSTALL_DIR ?= $(HOME)/.local/bin
BASH_COMPLETION_DIR ?= $(HOME)/.local/share/bash-completion/completions
ZSH_COMPLETION_DIR ?= $(HOME)/.local/share/zsh/site-functions
FISH_COMPLETION_DIR ?= $(HOME)/.config/fish/completions
MOCKERY ?= $(shell go env GOPATH)/bin/mockery

.PHONY: build install completions install-completions test mocks mockery-install fmt run clean

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .

completions: build
	@mkdir -p $(COMPLETIONS_DIR)
	$(BUILD_DIR)/$(BINARY_NAME) completion bash > $(COMPLETIONS_DIR)/$(BINARY_NAME).bash
	$(BUILD_DIR)/$(BINARY_NAME) completion zsh > $(COMPLETIONS_DIR)/_$(BINARY_NAME)
	$(BUILD_DIR)/$(BINARY_NAME) completion fish > $(COMPLETIONS_DIR)/$(BINARY_NAME).fish

install: build install-completions
	@mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)

install-completions: build
	@mkdir -p $(BASH_COMPLETION_DIR) $(ZSH_COMPLETION_DIR) $(FISH_COMPLETION_DIR)
	$(BUILD_DIR)/$(BINARY_NAME) completion bash > $(BASH_COMPLETION_DIR)/$(BINARY_NAME)
	$(BUILD_DIR)/$(BINARY_NAME) completion zsh > $(ZSH_COMPLETION_DIR)/_$(BINARY_NAME)
	$(BUILD_DIR)/$(BINARY_NAME) completion fish > $(FISH_COMPLETION_DIR)/$(BINARY_NAME).fish

fmt:
	gofmt -w main.go commands internal

mockery-install:
	go install github.com/vektra/mockery/v2@v2.53.5

mocks:
	$(MOCKERY) --dir internal --name API --output internal/mocks --outpkg mocks

test:
	go test ./...

run:
	go run .

clean:
	rm -rf $(BUILD_DIR)
