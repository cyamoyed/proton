APP_NAME := proton
BUILD_DIR := build
CMD_DIR := ./cmd/proton

# Version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

# Platforms
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: all build clean test vet fmt install build-all

all: build

## build: Build for the current platform
build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)$(if $(filter windows,$(GOOS)),.exe) $(CMD_DIR)
	@cp $(BUILD_DIR)/$(APP_NAME)$(if $(filter windows,$(GOOS)),.exe) $(BUILD_DIR)/$(APP_NAME)-installer$(if $(filter windows,$(GOOS)),.exe) 2>/dev/null || true
	@cp scripts/proton$(if $(filter windows,$(GOOS)),.bat,.sh) $(BUILD_DIR)/ 2>/dev/null || true
	@echo "Built: $(BUILD_DIR)/$(APP_NAME)"

## build-all: Build for all supported platforms
build-all: clean
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} ; \
		suffix="" ; \
		wrapper="proton.sh" ; \
		if [ "$$GOOS" = "windows" ]; then suffix=".exe"; wrapper="proton.bat"; fi ; \
		echo "Building $$GOOS/$$GOARCH..." ; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-$$GOOS-$$GOARCH$$suffix $(CMD_DIR) ; \
		cp $(BUILD_DIR)/$(APP_NAME)-$$GOOS-$$GOARCH$$suffix $(BUILD_DIR)/$(APP_NAME)-installer-$$GOOS-$$GOARCH$$suffix ; \
	done
	@echo "All builds complete in $(BUILD_DIR)/"

## install: Install to GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" $(CMD_DIR)

## test: Run tests
test:
	go test -count=1 ./...

## vet: Run go vet
vet:
	go vet ./...

## fmt: Format code
fmt:
	gofmt -w .

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## help: Show this help
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
