PREFIX ?= /usr/local
OMO_INSTALL_DIR ?= $(PREFIX)/bin
OMO_MANDIR ?= $(PREFIX)/share/man/man1
OMO_HOME := $(HOME)/.omo
PLUGINS_INSTALL_DIR := $(OMO_HOME)/plugins
# RPC plugin executables (hashicorp/go-plugin)
RPC_PLUGINS := redis docker git sysprocess argocd k8suser ssh postgres rabbitmq kafka github s3 awsCosts k8sportforward bunnydns dnscheck jira
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

.PHONY: all build install-omo clean install dirs plugin-redis plugin-docker plugin-git plugin-sysprocess plugin-argocd plugin-k8suser plugin-ssh plugin-postgres plugin-rabbitmq plugin-kafka plugin-github plugin-s3 plugin-awsCosts plugin-k8sportforward plugin-bunnydns plugin-dnscheck plugin-jira dev-setup dev-secrets dev-seed dev-up dev-down

# Build the host binary and all plugins, then install to ~/.omo
all: dirs
	@echo "Building omo $(VERSION)"
	@go mod tidy
	@go build $(LDFLAGS) -o omo ./cmd/omo
	@go install $(LDFLAGS) ./cmd/omo
	@echo "Building and installing plugins to $(PLUGINS_INSTALL_DIR)"
	@for name in $(RPC_PLUGINS); do \
		mkdir -p $(PLUGINS_INSTALL_DIR)/$$name; \
		echo "  $$name"; \
		go build -o $(PLUGINS_INSTALL_DIR)/$$name/$$name ./plugins/$$name/cmd/$$name; \
		chmod +x $(PLUGINS_INSTALL_DIR)/$$name/$$name; \
	done
	@echo "Generating installed manifest"
	@go run ./cmd/manifest
	@cp index.yaml $(OMO_HOME)/index.yaml
	@echo "Done. Plugins installed to $(PLUGINS_INSTALL_DIR)"

# Build host + plugins, then install omo onto PATH and man omo onto manpath.
build: all install-omo

# dest mode src name — copy src to dest/name, creating dest and using sudo if needed.
define omo-install
	@if [ ! -d "$(1)" ]; then mkdir -p "$(1)" 2>/dev/null || sudo mkdir -p "$(1)"; fi
	@if [ -w "$(1)" ]; then install -m $(2) $(3) "$(1)/$(4)"; \
	else echo "Installing $(4) to $(1) (sudo)..."; sudo install -m $(2) $(3) "$(1)/$(4)"; fi
endef

install-omo:
	@if [ ! -f omo ]; then echo "omo binary missing — run make all first"; exit 1; fi
	$(call omo-install,$(OMO_INSTALL_DIR),755,omo,omo)
	$(call omo-install,$(OMO_MANDIR),644,man/omo.1,omo.1)
	@echo "omo $(VERSION) → $(OMO_INSTALL_DIR)/omo"
	@echo "man omo        → $(OMO_MANDIR)/omo.1"
	@hash -r 2>/dev/null || true
	@if command -v omo >/dev/null 2>&1; then \
		echo "on PATH: $$(command -v omo)"; \
	else \
		echo "not on PATH — add $(OMO_INSTALL_DIR) to PATH"; \
	fi

# Build only the redis RPC plugin binary for fast iteration
plugin-redis: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/redis
	@echo "Building redis RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/redis/redis ./plugins/redis/cmd/redis
	@chmod +x $(PLUGINS_INSTALL_DIR)/redis/redis
	@echo "Installed $(PLUGINS_INSTALL_DIR)/redis/redis"

plugin-docker: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/docker
	@echo "Building docker RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/docker/docker ./plugins/docker/cmd/docker
	@chmod +x $(PLUGINS_INSTALL_DIR)/docker/docker
	@echo "Installed $(PLUGINS_INSTALL_DIR)/docker/docker"

plugin-git: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/git
	@echo "Building git RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/git/git ./plugins/git/cmd/git
	@chmod +x $(PLUGINS_INSTALL_DIR)/git/git
	@echo "Installed $(PLUGINS_INSTALL_DIR)/git/git"

plugin-sysprocess: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/sysprocess
	@echo "Building sysprocess RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/sysprocess/sysprocess ./plugins/sysprocess/cmd/sysprocess
	@chmod +x $(PLUGINS_INSTALL_DIR)/sysprocess/sysprocess
	@echo "Installed $(PLUGINS_INSTALL_DIR)/sysprocess/sysprocess"

plugin-github: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/github
	@echo "Building github RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/github/github ./plugins/github/cmd/github
	@chmod +x $(PLUGINS_INSTALL_DIR)/github/github
	@echo "Installed $(PLUGINS_INSTALL_DIR)/github/github"

plugin-s3: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/s3
	@echo "Building s3 RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/s3/s3 ./plugins/s3/cmd/s3
	@chmod +x $(PLUGINS_INSTALL_DIR)/s3/s3
	@echo "Installed $(PLUGINS_INSTALL_DIR)/s3/s3"

plugin-awsCosts: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/awsCosts
	@echo "Building awsCosts RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/awsCosts/awsCosts ./plugins/awsCosts/cmd/awsCosts
	@chmod +x $(PLUGINS_INSTALL_DIR)/awsCosts/awsCosts
	@echo "Installed $(PLUGINS_INSTALL_DIR)/awsCosts/awsCosts"

plugin-argocd: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/argocd
	@echo "Building argocd RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/argocd/argocd ./plugins/argocd/cmd/argocd
	@chmod +x $(PLUGINS_INSTALL_DIR)/argocd/argocd
	@echo "Installed $(PLUGINS_INSTALL_DIR)/argocd/argocd"

plugin-k8suser: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/k8suser
	@echo "Building k8suser RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/k8suser/k8suser ./plugins/k8suser/cmd/k8suser
	@chmod +x $(PLUGINS_INSTALL_DIR)/k8suser/k8suser
	@echo "Installed $(PLUGINS_INSTALL_DIR)/k8suser/k8suser"

plugin-ssh: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/ssh
	@echo "Building ssh RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/ssh/ssh ./plugins/ssh/cmd/ssh
	@chmod +x $(PLUGINS_INSTALL_DIR)/ssh/ssh
	@echo "Installed $(PLUGINS_INSTALL_DIR)/ssh/ssh"

plugin-postgres: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/postgres
	@echo "Building postgres RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/postgres/postgres ./plugins/postgres/cmd/postgres
	@chmod +x $(PLUGINS_INSTALL_DIR)/postgres/postgres
	@echo "Installed $(PLUGINS_INSTALL_DIR)/postgres/postgres"

plugin-rabbitmq: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/rabbitmq
	@echo "Building rabbitmq RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/rabbitmq/rabbitmq ./plugins/rabbitmq/cmd/rabbitmq
	@chmod +x $(PLUGINS_INSTALL_DIR)/rabbitmq/rabbitmq
	@echo "Installed $(PLUGINS_INSTALL_DIR)/rabbitmq/rabbitmq"

plugin-kafka: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/kafka
	@echo "Building kafka RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/kafka/kafka ./plugins/kafka/cmd/kafka
	@chmod +x $(PLUGINS_INSTALL_DIR)/kafka/kafka
	@echo "Installed $(PLUGINS_INSTALL_DIR)/kafka/kafka"

plugin-k8sportforward: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/k8sportforward
	@echo "Building k8sportforward RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/k8sportforward/k8sportforward ./plugins/k8sportforward/cmd/k8sportforward
	@chmod +x $(PLUGINS_INSTALL_DIR)/k8sportforward/k8sportforward
	@echo "Installed $(PLUGINS_INSTALL_DIR)/k8sportforward/k8sportforward"

plugin-bunnydns: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/bunnydns
	@echo "Building bunnydns RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/bunnydns/bunnydns ./plugins/bunnydns/cmd/bunnydns
	@chmod +x $(PLUGINS_INSTALL_DIR)/bunnydns/bunnydns
	@echo "Installed $(PLUGINS_INSTALL_DIR)/bunnydns/bunnydns"

plugin-dnscheck: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/dnscheck
	@echo "Building dnscheck RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/dnscheck/dnscheck ./plugins/dnscheck/cmd/dnscheck
	@chmod +x $(PLUGINS_INSTALL_DIR)/dnscheck/dnscheck
	@echo "Installed $(PLUGINS_INSTALL_DIR)/dnscheck/dnscheck"

plugin-jira: dirs
	@mkdir -p $(PLUGINS_INSTALL_DIR)/jira
	@echo "Building jira RPC plugin"
	@go build -o $(PLUGINS_INSTALL_DIR)/jira/jira ./plugins/jira/cmd/jira
	@chmod +x $(PLUGINS_INSTALL_DIR)/jira/jira
	@echo "Installed $(PLUGINS_INSTALL_DIR)/jira/jira"

# Create the ~/.omo directory structure
dirs:
	@mkdir -p $(PLUGINS_INSTALL_DIR)

# Seed KeePass secrets for all plugins.
# Plugins that need Docker (redis, kafka, postgres, rabbitmq) also start containers.
dev-setup:
	@bash dev/setup.sh

# Start local Docker stacks + kind (redis, kafka, postgres, rabbitmq, k8suser/argocd).
dev-up:
	@bash dev/up.sh

# Stop local Docker stacks + kind cluster started by dev-up.
dev-down:
	@bash dev/down.sh

# Clean placeholder/smoke KeePass entries and reseed local/dev credentials only.
dev-secrets:
	@go run dev/clean_and_seed.go

# Alias for dev-secrets.
dev-seed:
	@go run dev/clean_and_seed.go

clean:
	@rm -f omo
	@echo "Note: installed plugins at $(PLUGINS_INSTALL_DIR) are NOT removed."
	@echo "Run 'make purge' to remove everything."

purge:
	@rm -rf $(OMO_HOME)
	@rm -f omo
	@echo "Removed $(OMO_HOME) and omo binary"
