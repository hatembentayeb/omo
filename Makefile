PLUGINS_DIR := ./plugins
COMPILED_PLUGINS_DIR := ./compiled_plugins
BUILD_MODE := -buildmode=plugin

.PHONY: all clean

all:
	@echo "Building omo"
	@go mod tidy
	@go build -o omo ./cmd/omo
	@go install ./cmd/omo
	@echo "Building plugins"
	@mkdir -p $(COMPILED_PLUGINS_DIR)
	@for plugin in $(wildcard $(PLUGINS_DIR)/*); do \
		echo "Building $$(basename $$plugin) plugin"; \
		go build $(BUILD_MODE) -o $(COMPILED_PLUGINS_DIR)/$$(basename $$plugin) $$plugin; \
	done

clean:
	@rm -rf $(COMPILED_PLUGINS_DIR)
