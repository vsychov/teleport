include install.mk

LOCALDIR := $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))
PROTOC_GEN_TERRAFORM:= $(shell command -v protoc-gen-terraform)

BUILDDIR ?= build
TFDIR ?= example

ADDFLAGS ?=
BUILDFLAGS ?= $(ADDFLAGS) -trimpath -ldflags '-w -s'
# CGO must NOT be enabled as hashicorp cloud does not support running providers using on CGO.
CGOFLAG ?= CGO_ENABLED=0

RELEASE = terraform-provider-teleport-v$(VERSION)-$(OS)-$(ARCH)-bin

.PHONY: tfclean
tfclean:
	rm -rf $(TFDIR)/terraform.tfstate
	rm -rf $(TFDIR)/terraform.tfstate.backup
	rm -rf $(TFDIR)/.terraform
	rm -rf $(TFDIR)/.terraform.lock.hcl

.PHONY: clean
clean: tfclean
	rm -rf $(PROVIDER_PATH)*
	rm -rf $(BUILDDIR)/*
	rm -rf $(RELEASE).tar.gz
	go clean

.PHONY: build
build: clean
# Turning off GOWORK to prevent missing package errors.
	GOWORK=off GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "kustomize_disable_go_plugin_support" -o $(BUILDDIR)/terraform-provider-teleport $(BUILDFLAGS)

build-darwin-universal: $(addprefix $(BUILDDIR)/terraform-provider-teleport_,arm64 amd64)
	lipo -create -output $(BUILDDIR)/terraform-provider-teleport $^
	rm -r $^ $(BUILDDIR)/$(OS)

# Architecture-specific binaries for the universal binary are extracted from
# the release tarball. make will not automatically build this; you will need
# to run "make ARCH=amd64 release" and "make ARCH=arm64 release" first as is
# done in the build workflow.
$(BUILDDIR)/terraform-provider-teleport_%: terraform-provider-teleport-v$(VERSION)-$(OS)-%-bin.tar.gz
	mkdir -p $(BUILDDIR)/$(OS)/$*
	tar -xzf $< -C $(BUILDDIR)/$(OS)/$*
	mv $(BUILDDIR)/$(OS)/$*/terraform-provider-teleport $@

CUSTOM_IMPORTS_TMP_DIR ?= /tmp/protoc-gen-terraform/custom-imports
# This version must match the version installed by .github/workflows/lint.yaml
PROTOC_GEN_TERRAFORM_VERSION ?= v3.0.2
PROTOC_GEN_TERRAFORM_EXISTS := $(shell $(PROTOC_GEN_TERRAFORM) version 2>&1 >/dev/null | grep 'protoc-gen-terraform $(PROTOC_GEN_TERRAFORM_VERSION)')

.PHONY: gen-tfschema
gen-tfschema:
ifndef PROTOC_GEN_TERRAFORM_EXISTS
	@echo "protoc-gen-terraform $(PROTOC_GEN_TERRAFORM_VERSION) is not installed. Please, refer to README.md for installation instructions."
	@exit -1
endif

# The wrappers.proto file needed for this generator exist only inside the go mod cache,
# so we retrieve the file path for the cached proto files with go mod tools.
	$(eval PROTOBUF_MOD_PATH := $(shell go list -m -u -f '{{.Dir}}' github.com/gogo/protobuf))

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(PROTOC_GEN_TERRAFORM) \
		--terraform_out=config=protoc-gen-terraform-teleport.yaml:./tfschema \
		teleport/legacy/types/types.proto

	mv ./tfschema/github.com/gravitational/teleport/api/types/types_terraform.go ./tfschema/

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(PROTOC_GEN_TERRAFORM) \
		--terraform_out=config=protoc-gen-terraform-loginrule.yaml:./tfschema \
		teleport/loginrule/v1/loginrule.proto

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(PROTOC_GEN_TERRAFORM) \
		--terraform_out=config=protoc-gen-terraform-devicetrust.yaml:./tfschema \
		teleport/legacy/types/device.proto

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(PROTOC_GEN_TERRAFORM) \
		--terraform_out=config=protoc-gen-terraform-accesslist.yaml:./tfschema \
		teleport/accesslist/v1/accesslist.proto

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(PROTOC_GEN_TERRAFORM) \
		--terraform_out=config=protoc-gen-terraform-accessmonitoringrules.yaml:./tfschema \
		teleport/accessmonitoringrules/v1/access_monitoring_rules.proto

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(PROTOC_GEN_TERRAFORM) \
		--terraform_out=config=protoc-gen-terraform-statichostuser.yaml:./tfschema \
		teleport/userprovisioning/v2/statichostuser.proto

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(PROTOC_GEN_TERRAFORM) \
		--terraform_out=config=protoc-gen-terraform-workloadidentity.yaml:./tfschema \
		teleport/workloadidentity/v1/resource.proto

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(PROTOC_GEN_TERRAFORM) \
		--terraform_out=config=protoc-gen-terraform-autoupdate.yaml:./tfschema \
		teleport/autoupdate/v1/autoupdate.proto

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(PROTOC_GEN_TERRAFORM) \
		--terraform_out=config=protoc-gen-terraform-healthcheckconfig.yaml:./tfschema \
		teleport/healthcheckconfig/v1/health_check_config.proto

	mv ./tfschema/github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1/loginrule_terraform.go ./tfschema/loginrule/v1/
	mv ./tfschema/github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1/accesslist_terraform.go ./tfschema/accesslist/v1/
	mv ./tfschema/github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1/access_monitoring_rules_terraform.go ./tfschema/accessmonitoringrules/v1/
	mv ./tfschema/github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2/statichostuser_terraform.go ./tfschema/userprovisioning/v2/
	mv ./tfschema/github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1/resource_terraform.go ./tfschema/workloadidentity/v1/
	mv ./tfschema/github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1/autoupdate_terraform.go ./tfschema/autoupdate/v1/
	mv ./tfschema/github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1/health_check_config_terraform.go ./tfschema/healthcheckconfig/v1/
	mv ./tfschema/github.com/gravitational/teleport/api/types/device_terraform.go ./tfschema/devicetrust/v1/
	rm -r ./tfschema/github.com/
	@go run ./gen/main.go

# darwin-signed-build is a wrapper around the build target that ensures it is codesigned
include ../../darwin-signing.mk
.PHONY: darwin-signed-build
darwin-signed-build: BINARIES=$(BUILDDIR)/terraform-provider-teleport
ifeq ($(OS)-$(ARCH),darwin-universal)
darwin-signed-build: build-darwin-universal
else
darwin-signed-build: build
endif
	$(NOTARIZE_BINARIES)

.PHONY: release
ifeq ($(OS),darwin)
release: darwin-signed-build
else
release: build
endif
	tar -C $(BUILDDIR) -czf $(RELEASE).tar.gz .

TERRAFORM_EXISTS := $(shell terraform -version 2>/dev/null | grep 'Terraform v1.')
CURRENT_ULIMIT := $(shell ulimit -n)

TEST_ARGS?=

.PHONY: test
test: install
ifndef TERRAFORM_EXISTS
	@echo "Terraform v1.4+ is not installed (tfenv install 1.5.6 && tfenv use 1.5.6)."
	terraform -version
	@exit -1
endif
# NOTE: This is related to an old bug in Terraform and will be fixed in future releases (possibly, by a workaround on our side)
ifeq ($(shell expr $(CURRENT_ULIMIT) \< 1024), 1)
	@echo "ulimit -n is too low ($(CURRENT_ULIMIT)), please set ulimit -n 1024"
	@exit -1
endif
	go test ./testlib -v $(TEST_ARGS)

.PHONY: test-full
test-full: TEST_ARGS=--tags enterprisetests
test-full: test

.PHONY: test-ent
test-ent: TEST_ARGS=--tags enterprisetests -run 'TestTerraformEnterprise.*'
test-ent: test

.PHONY: apply
apply: install
	terraform -chdir=$(TFDIR) init && terraform -chdir=$(TFDIR) apply -auto-approve

.PHONY: reapply
reapply:
	terraform -chdir=$(TFDIR) apply

.PHONY: destroy
destroy:
	terraform -chdir=$(TFDIR) destroy -auto-approve

.PHONY: lint
lint:
	golangci-lint run -c ../../.golangci.yml

.PHONY: fmt
fmt:
	terraform fmt -no-color -recursive

.PHONY: docs
docs: gen-tfschema install fmt
	./gen/docs.sh $(VERSION)
