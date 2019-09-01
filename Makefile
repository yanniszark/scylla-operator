
# Image URL to use all building/pushing image targets
REPO ?= "yanniszark/scylla-operator"
TAG ?= "v0.0-$(shell git rev-parse --short HEAD)"
IMG ?= "${REPO}:${TAG}"
GO ?= GO111MODULE=off go

all: test local-build

# Run tests
test: generate fmt vet manifests vendor
	$(GO) test ./pkg/... ./cmd/... -coverprofile cover.out

# Build local-build binary
local-build: generate fmt vet vendor
	$(GO) build -o bin/manager github.com/scylladb/scylla-operator/cmd

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet vendor
	$(GO) run ./cmd operator --image="${IMG}" --enable-admission-webhook=false

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: install
	kubectl apply -f config/rbac
	kustomize build config | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	$(GO) run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
	cd config && kustomize edit set image yanniszark/scylla-operator="${IMG}"
	kustomize build config > examples/generic/operator.yaml
	kustomize build config > examples/gke/operator.yaml
	kustomize build config > examples/minikube/operator.yaml

# Run go fmt against code
fmt:
	$(GO) fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	$(GO) vet ./pkg/... ./cmd/...

# Generate code
generate:
	$(GO) generate ./pkg/... ./cmd/...

# Ensure dependencies
vendor:
	dep ensure -v

# Build the docker image
docker-build: test
	docker build . -t "${IMG}"

# Push the docker image
docker-push:
	docker push "${IMG}"

publish: docker-build docker-push