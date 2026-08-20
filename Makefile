.SUFFIXES:
.DEFAULT_GOAL := help

.PHONY: help
help:  ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5)} /^[a-zA-Z_.\/-]+:.*## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: setup-lint
setup-lint: install-golangci-lint install-shellcheck  ## Install lint tools (golangci-lint, shellcheck)

.PHONY: lint
lint: lint-golangci-lint lint-shellcheck  ## Run all linters

.PHONY:	lint-golangci-lint
lint-golangci-lint: install-golangci-lint  ## Run golangci-lint
	golangci-lint run -c .github/.golangci.yaml

.PHONY: lint-shellcheck
lint-shellcheck: install-shellcheck lint-shellcheck-spread  ## Run shellcheck on all shell scripts
	git ls-files | file --mime-type -Nnf- | grep shellscript | cut -f1 -d: | xargs -r shellcheck

.PHONY: lint-shellcheck-spread
lint-shellcheck-spread:
	shellcheck spread/.extension
	spread/tools/utils/spread-shellcheck spread.yaml
	find spread -name task.yaml | xargs spread/tools/utils/spread-shellcheck

.PHONY: install-golangci-lint
install-golangci-lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		if command -v snap >/dev/null 2>&1; then \
			sudo snap install --classic golangci-lint; \
		else \
			echo "Error: golangci-lint not found. Please install it manually." >&2; \
			exit 1; \
		fi \
	fi

.PHONY: install-shellcheck
install-shellcheck:
	@if ! command -v shellcheck >/dev/null 2>&1; then \
		if command -v snap >/dev/null 2>&1; then \
			sudo snap install --classic shellcheck; \
		else \
			echo "Error: shellcheck not found. Please install it manually." >&2; \
			exit 1; \
		fi \
	fi
