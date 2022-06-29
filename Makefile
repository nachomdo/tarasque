# ====================================================================================
# Setup Project
PROJECT_NAME := tarasque
PROJECT_REPO := github.com/nachomdo/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64
-include build/makelib/common.mk

# Setup Output
-include build/makelib/output.mk

# Setup Go
NPROCS ?= 1
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal apis
GO111MODULE = on
-include build/makelib/golang.mk

# Setup Kubernetes tools
-include build/makelib/k8s_tools.mk

# Setup Helm
USE_HELM3 = true
# HELM_BASE_URL = https://charts.upbound.io
# HELM_CHART_LINT_STRICT = false
# HELM_S3_BUCKET = nacho-ccloud-test
# HELM_CHARTS = tarasque
# HELM_CHART_LINT_ARGS_tarasque = --set nameOverride='',imagePullSecrets=''
# -include build/makelib/helm.mk

# Setup Images
DOCKER_REGISTRY ?= nachomdo
IMAGES = $(PROJECT_NAME) $(PROJECT_NAME)-controller
-include build/makelib/image.mk

#Local deployment
-include build/makelib/deploy.mk
-include build/makelib/local.mk
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

crds.clean:
	@$(INFO) cleaning generated CRDs
	@find package/crds -name *.yaml -exec sed -i.sed -e '1,2d' {} \; || $(FAIL)
	@find package/crds -name *.yaml.sed -delete || $(FAIL)
	@$(OK) cleaned generated CRDs

generate: crds.clean

# integration tests
e2e.run: test-integration

# Run integration tests.
test-integration: $(KIND) $(KUBECTL) $(HELM3)
	@$(INFO) running integration tests using kind $(KIND_VERSION)
	@$(ROOT_DIR)/cluster/local/integration_tests.sh || $(FAIL)
	@$(OK) integration tests passed

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# NOTE(hasheddan): the build submodule currently overrides XDG_CACHE_HOME in
# order to force the Helm 3 to use the .work/helm directory. This causes Go on
# Linux machines to use that directory as the build cache as well. We should
# adjust this behavior in the build submodule because it is also causing Linux
# users to duplicate their build cache, but for now we just make it easier to
# identify its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: go.build
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	@# To see other arguments that can be provided, run the command with --help instead
	$(GO_OUT_DIR)/$(PROJECT_NAME) --debug

dev: $(KIND) $(KUBECTL)
#	@$(INFO) Creating kind cluster
#	@$(KIND) create cluster --name=$(PROJECT_NAME)-dev
#	@$(KUBECTL) cluster-info --context kind-$(PROJECT_NAME)-dev
	@$(INFO) Installing Crossplane CRDs
	@$(KUBECTL) apply -k https://github.com/crossplane/crossplane//cluster?ref=master
	@$(INFO) Installing Provider Tarasque CRDs
	@$(KUBECTL) apply -R -f package/crds
	@$(INFO) Starting Provider Tarasque controllers
	@$(GO) run cmd/provider/main.go --debug

install: $(KUBECTL) $(HELM3)
	@$(INFO) Deploying Tarasque to Kubernetes cluster
	@$(INFO) Installing Crossplane
	@$(HELM3) repo add crossplane-stable https://charts.crossplane.io/stable
	@$(HELM3) repo update
	@$(HELM3) upgrade --install crossplane --create-namespace --namespace crossplane-system crossplane-stable/crossplane
	@$(INFO) Installing Crossplane CRDs
	@$(KUBECTL) apply -k https://github.com/crossplane/crossplane/cluster?ref=master
	@$(INFO) Installing Provider Tarasque CRDs
	@$(KUBECTL) apply -R -f package/crds
	@$(KUBECTL) crossplane install provider nachomdo/provider-tarasque-controller:v0.7
	@$(INFO) Deploying Trogdor Agents 
	@$(KUBECTL) apply -f manifests/manifest.yaml
	@$(KUBECTL) apply -f examples/provider/config.yaml
uninstall: $(KUBECTL) $(HELM3)
	@$(INFO) Removing Tarasque from Kubernetes cluster 
	@$(KUBECTL) delete -f examples/provider/config.yaml
	@$(KUBECTL) delete -f manifests/manifest.yaml 
	@$(KUBECTL) delete -R -f package/crds 
	@$(KUBECTL) delete providers nachomdo-provider-tarasque-controller
	@$(HEML3) uninstall crossplane --namespace crossplane-system 

dev-clean: $(KIND) $(KUBECTL)
	@$(INFO) Deleting kind cluster
	@$(KIND) delete cluster --name=$(PROJECT_NAME)-dev

deploy: build
	@$(INFO) Deploying to active cluster
	@docker tag build-533b5664/provider-template-controller-amd64 nachomdo/tarasque-controller:v0.7
	@docker push nachomdo/tarasque-controller:v0.7
	@rm -rf package/*.xpkg
	@$(KUBECTL) crossplane build provider -f package/ --name tarasque
	@$(KUBECTL) crossplane push provider nachomdo/provider-tarasque-controller:v0.7 -f package/tarasque.xpkg
	@$(KUBECTL) crossplane update provider nachomdo-provider-tarasque-controller v0.7
	@$(KUBECTL) delete pod --force -l pkg.crossplane.io/provider=tarasque -n crossplane-system

.PHONY: submodules fallthrough test-integration run crds.clean dev dev-clean deploy

# ====================================================================================
# Special Targets

define CROSSPLANE_MAKE_HELP
Crossplane Targets:
    submodules            Update the submodules, such as the common build scripts.
    run                   Run crossplane locally, out-of-cluster. Useful for development.

endef
# The reason CROSSPLANE_MAKE_HELP is used instead of CROSSPLANE_HELP is because the crossplane
# binary will try to use CROSSPLANE_HELP if it is set, and this is for something different.
export CROSSPLANE_MAKE_HELP

crossplane.help:
	@echo "$$CROSSPLANE_MAKE_HELP"

help-special: crossplane.help

.PHONY: crossplane.help help-special
