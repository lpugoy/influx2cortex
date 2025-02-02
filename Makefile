.PHONY: help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build local binaries
	bash ./scripts/compile_commands.sh

build-local: ## Builds local versions of images
	bash ./scripts/build-local-images.sh

acceptance-tests: ## Runs the acceptance tests, expecting the images to be already built
	go test -count 1 -v -race -tags=acceptance ./acceptance/...

test: ## Run golang tests
	go test -race ./...

coverage-output:
	go test ./... -coverprofile=cover.out

coverage-show-func:
	go tool cover -func cover.out

# .PHONY: build
# build: ## Build the grpc-cortex-gw docker image
# build:
# 	docker build --build-arg=revision=$(GIT_REVISION) -t jdbgrafana/grpc-cortex-gw .

# CI
drone:
	scripts/generate-drone-yml.sh

drone-utilities:
	scripts/build-drone-utilities.sh
