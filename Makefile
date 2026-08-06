PLUGINS_DIR := ./plugins
OMO_HOME := $(HOME)/.omo
PLUGINS_INSTALL_DIR := $(OMO_HOME)/plugins
BUILD_MODE := -buildmode=plugin
# Plugins built as RPC executables (hashicorp/go-plugin) instead of native .so
RPC_PLUGINS := redis
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

.PHONY: all clean install dirs plugin-redis

# Build the host binary and all plugins, then install to ~/.omo
all: dirs
	@echo "Building omo $(VERSION)"
	@go mod tidy
	@go build $(LDFLAGS) -o omo ./cmd/omo
	@go install $(LDFLAGS) ./cmd/omo
	@echo "Building and installing plugins to $(PLUGINS_INSTALL_DIR)"
	@for plugin in $(wildcard $(PLUGINS_DIR)/*); do \
		name=$$(basename $$plugin); \
		mkdir -p $(PLUGINS_INSTALL_DIR)/$$name; \
		if echo "$(RPC_PLUGINS)" | grep -qw "$$name"; then \
			echo "  $$name (rpc)"; \
			go build -o $(PLUGINS_INSTALL_DIR)/$$name/$$name ./plugins/$$name/cmd/$$name; \
			chmod +x $(PLUGINS_INSTALL_DIR)/$$name/$$name; \
		else \
			echo "  $$name (native)"; \
			go build $(BUILD_MODE) -o $(PLUGINS_INSTALL_DIR)/$$name/$$name.so $$plugin; \
		fi; \
	done
	@echo "Generating installed manifest"
	@go run ./cmd/manifest
	@cp index.yaml $(OMO_HOME)/index.yaml
	@echo "Done. Plugins installed to $(PLUGINS_INSTALL_DIR)"

# Build only the redis RPC plugin binary for fast iteration
plugin-redis: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/redis
	@echo "Building redis RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/redis/redis ./plugins/redis/cmd/redis
	@chmod +x $(PLUGINS_INSTALL_DIR)/redis/redis
	@echo "Installed $(PLUGINS_INSTALL_DIR)/redis/redis"

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
