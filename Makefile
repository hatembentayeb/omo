PLUGINS_DIR := ./plugins
OMO_HOME := $(HOME)/.omo
PLUGINS_INSTALL_DIR := $(OMO_HOME)/plugins
BUILD_MODE := -buildmode=plugin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

.PHONY: all clean install dirs

# Build the host binary and all plugins, then install to ~/.omo
all: dirs
	@echo "Building omo $(VERSION)"
	@go mod tidy
	@go build $(LDFLAGS) -o omo ./cmd/omo
	@go install $(LDFLAGS) ./cmd/omo
	@echo "Building and installing plugins to $(PLUGINS_INSTALL_DIR)"
	@for plugin in $(wildcard $(PLUGINS_DIR)/*); do \
		name=$$(basename $$plugin); \
		echo "  $$name"; \
		mkdir -p $(PLUGINS_INSTALL_DIR)/$$name; \
		go build $(BUILD_MODE) -o $(PLUGINS_INSTALL_DIR)/$$name/$$name.so $$plugin; \
	done
	@echo "Generating installed manifest"
	@go run ./cmd/manifest
	@cp index.yaml $(OMO_HOME)/index.yaml
	@echo "Done. Plugins installed to $(PLUGINS_INSTALL_DIR)"

# Create the ~/.omo directory structure
dirs:
	@mkdir -p $(PLUGINS_INSTALL_DIR)

# Seed KeePass secrets for all plugins.
# Plugins that need Docker (redis, kafka) also start their containers.
dev-setup:
	@bash dev/setup.sh

# Seed KeePass secrets for plugins that don't need Docker.
dev-seed:
	@for plugin in docker git awsCosts s3 k8suser argocd; do \
		setup="dev/$$plugin/setup.sh"; \
		if [ -f "$$setup" ]; then \
			echo "==> Seeding $$plugin"; \
			bash "$$setup"; \
			echo ""; \
		fi; \
	done

clean:
	@rm -f omo
	@echo "Note: installed plugins at $(PLUGINS_INSTALL_DIR) are NOT removed."
	@echo "Run 'make purge' to remove everything."

purge:
	@rm -rf $(OMO_HOME)
	@rm -f omo
	@echo "Removed $(OMO_HOME) and omo binary"
