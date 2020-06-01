# build command:
# REGISTRY=<registry_url> TAG="<tag_name>" make all

include variables.mk

NODE            := node
DRIVE_MANAGER   := drivemgr
CONTROLLER      := controller
HEALTH_PROBE    := health_probe

GO_ENV_VARS  := GO111MODULE=on GOPRIVATE=eos2git.cec.lab.emc.com/* GOPROXY=http://asdrepo.isus.emc.com/artifactory/api/go/ecs-go-build,https://proxy.golang.org,direct

.PHONY: version test build install-hal

# print version
version:
	@printf $(TAG)

#all: build image push

# use in clear environment
prepare-env: install-compile-proto install-hal install-controller-gen dependency generate-api

dependency:
	${GO_ENV_VARS} go mod download

build: compile-proto build-drivemgr build-node build-controller

# NOTE: Output directory for binary file should be in Docker context.
# So we can't use /baremetal-csi-plugin/build to build the image.
build-drivemgr:
	go build -o ./build/${DRIVE_MANAGER}/${DRIVE_MANAGER} ./cmd/${DRIVE_MANAGER}/main.go

build-node:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${NODE}/${NODE} ./cmd/${NODE}/main.go

build-controller:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${CONTROLLER}/${CONTROLLER} ./cmd/${CONTROLLER}/main.go

image: image-drivemgr image-node image-controller

base-image: base-image-drivemgr base-image-node base-image-controller

base-image-drivemgr:
	docker build --network host --force-rm --file ./pkg/${DRIVE_MANAGER}/Dockerfile.build --tag ${DRIVE_MANAGER}:base ./pkg/${DRIVE_MANAGER}

base-image-node: download-grpc-health-probe
	cp ./build/${HEALTH_PROBE} ./pkg/${NODE}/${HEALTH_PROBE}
	docker build --network host --force-rm --file ./pkg/${NODE}/Dockerfile.build --tag ${NODE}:base ./pkg/${NODE}

base-image-controller:
	docker build --network host --force-rm --file ./pkg/${CONTROLLER}/Dockerfile.build --tag ${CONTROLLER}:base ./pkg/${CONTROLLER}

image-drivemgr:
	cp ./build/${DRIVE_MANAGER}/${DRIVE_MANAGER} ./pkg/${DRIVE_MANAGER}/${DRIVE_MANAGER}
	docker build --network host --force-rm --tag ${REGISTRY}/${REPO}-${DRIVE_MANAGER}:${TAG} ./pkg/${DRIVE_MANAGER}
	docker tag ${REGISTRY}/${REPO}-${DRIVE_MANAGER}:${TAG} ${HARBOR}/${REPO}-${DRIVE_MANAGER}:${TAG}

image-node:
	cp ./build/${NODE}/${NODE} ./pkg/${NODE}/${NODE}
	docker build --network host --force-rm --tag ${REGISTRY}/${REPO}-${NODE}:${TAG} ./pkg/${NODE}
	docker tag ${REGISTRY}/${REPO}-${NODE}:${TAG} ${HARBOR}/${REPO}-${NODE}:${TAG}

image-controller:
	cp ./build/${CONTROLLER}/${CONTROLLER} ./pkg/${CONTROLLER}/${CONTROLLER}
	docker build --network host --force-rm --tag ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG} ./pkg/${CONTROLLER}
	docker tag ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG} ${HARBOR}/${REPO}-${CONTROLLER}:${TAG}

push: push-drivemgr push-node push-controller

push-local:
	docker push ${REGISTRY}/${REPO}-${DRIVE_MANAGER}:${TAG}
	docker push ${REGISTRY}/${REPO}-${NODE}:${TAG}
	docker push ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG}

push-drivemgr:
	docker push ${REGISTRY}/${REPO}-${DRIVE_MANAGER}:${TAG}
	docker push ${HARBOR}/${REPO}-${DRIVE_MANAGER}:${TAG}

push-node:
	docker push ${REGISTRY}/${REPO}-${NODE}:${TAG}
	docker push ${HARBOR}/${REPO}-${NODE}:${TAG}

push-controller:
	docker push ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG}
	docker push ${HARBOR}/${REPO}-${CONTROLLER}:${TAG}

# todo how to handle tag change for sidecars in charts?
kind-load-images:
	kind load docker-image csi-provisioner:v1.2.2
	kind load docker-image csi-node-driver-registrar:v1.0.1-gke.0
	kind load docker-image csi-attacher:v1.0.1
	kind load docker-image ${REPO}-${DRIVE_MANAGER}:${TAG}
	kind load docker-image ${REPO}-${NODE}:${TAG}
	kind load docker-image ${REPO}-${CONTROLLER}:${TAG}
	kind load docker-image busybox:1.29

kind-pull-images:
	docker pull ${REGISTRY}/csi-provisioner:v1.2.2
	docker pull ${REGISTRY}/csi-node-driver-registrar:v1.0.1-gke.0
	docker pull busybox:1.29
	docker pull ${REGISTRY}/csi-attacher:v1.0.1
	docker pull ${REGISTRY}/${REPO}-${DRIVE_MANAGER}:${TAG}
	docker pull ${REGISTRY}/${REPO}-${NODE}:${TAG}
	docker pull ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG}

kind-tag-images:
	docker tag ${REGISTRY}/csi-provisioner:v1.2.2 csi-provisioner:v1.2.2
	docker tag ${REGISTRY}/csi-node-driver-registrar:v1.0.1-gke.0 csi-node-driver-registrar:v1.0.1-gke.0
	docker tag ${REGISTRY}/csi-attacher:v1.0.1 csi-attacher:v1.0.1
	docker tag ${REGISTRY}/${REPO}-${DRIVE_MANAGER}:${TAG} ${REPO}-${DRIVE_MANAGER}:${TAG}
	docker tag ${REGISTRY}/${REPO}-${NODE}:${TAG} ${REPO}-${NODE}:${TAG}
	docker tag ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG} ${REPO}-${CONTROLLER}:${TAG}

clean: clean-drivemgr clean-node clean-controller clean-proto

clean-drivemgr:
	rm -rf ./build/${DRIVE_MANAGER}/${DRIVE_MANAGER}

clean-node:
	rm -rf ./build/${NODE}/${NODE}

clean-controller:
	rm -rf ./build/${CONTROLLER}/${CONTROLLER}

clean-proto:
	rm -rf ./api/generated/v1/*

clean-image: clean-image-drivemgr clean-image-node # clean-image-controller

clean-image-drivemgr:
	docker rmi ${REGISTRY}/${REPO}-${DRIVE_MANAGER}:${TAG}

clean-image-node:
	docker rmi ${REGISTRY}/${REPO}-${NODE}:${TAG}

clean-image-controller:
	docker rmi ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG}

lint:
	${GO_ENV_VARS} golangci-lint -v run ./...

lint-charts:
	helm lint ./${CHARTS_PATH}

test:
	${GO_ENV_VARS} go test -race -cover ./... -coverprofile=coverage.out

# Run tests for pr-validation with writing output to log file
# Test are different (ginkgo, go testing, etc.) so can't use native ginkgo methods to print junit output.
test-pr-validation:
	${GO_ENV_VARS} go test -v -race -cover ./... -coverprofile=coverage.out > log.txt

# Convert go test output from log.txt to junit-style output. Split these steps because no matter if test fails,
# junit output must be collected
pr-validation-junit:
	cat log.txt >&1 | go-junit-report > report.xml

# Run e2e tests for CI. All of these tests use ginkgo so we can use native ginkgo methods to print junit output.
# Also go test doesn't provide functionnality to save test's log into the file. Use > to archieve artifatcs.
test-ci:
	${GO_ENV_VARS} CI=true go test -v test/e2e/baremetal_e2e_test.go -ginkgo.v -ginkgo.progress -kubeconfig=${HOME}/.kube/config -timeout=0 > log.txt

# Run commnity sanity tests for CSI.
# TODO AK8S-640 Must fix tests "Node Service should be idempotent" and "Node Service should work"
test-sanity:
	${GO_ENV_VARS} SANITY=true go test test/sanity/sanity_test.go -ginkgo.skip \
	"ValidateVolumeCapabilities|\
	should fail when the node does not exist|\
	should fail when requesting to create a volume with already existing name and different capacity|\
	should not fail when requesting to create a volume with already existing name and same capacity" -ginkgo.v -timeout=0

install-junit-report:
	${GO_ENV_VARS} go get -u github.com/jstemmer/go-junit-report

coverage:
	go tool cover -html=coverage.out -o coverage.html

prepare-protoc:  # TODO: temporary solution while we build all in common devkit (DELIVERY-1488)
	mkdir -p proto_3.11.0
	curl -L -O https://github.com/protocolbuffers/protobuf/releases/download/v3.11.0/protoc-3.11.0-linux-x86_64.zip && \
	unzip protoc-3.11.0-linux-x86_64.zip -d proto_3.11.0/ && \
	sudo mv proto_3.11.0/bin/protoc /usr/bin/protoc && \
	protoc --version; rm -rf proto_3.11.0; rm protoc-*
	go get -u github.com/golang/protobuf/protoc-gen-go@v1.3.5

compile-proto:
	mkdir -p api/generated/v1/
	protoc -I=api/v1 --go_out=plugins=grpc:api/generated/v1 api/v1/*.proto

install-compile-proto: prepare-protoc compile-proto

install-hal:
	# NOTE: Root privileges are required for installing or uninstalling packages.
	sudo zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends http://asdrepo.isus.emc.com:8081/artifactory/ecs-prerelease-local/com/emc/asd/vipr/sles12/viprhal/viprhal/viprhal-${HAL_VERSION}-go.SLES/viprhal-${HAL_VERSION}.SLES.x86_64.rpm
	sudo zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends http://asdrepo.isus.emc.com:8081/artifactory/ecs-prerelease-local/com/emc/asd/vipr/sles12/viprhal/viprhal-devel/viprhal-${HAL_VERSION}-go.SLES/viprhal-devel-${HAL_VERSION}.SLES.x86_64.rpm

install-controller-gen:
	# Generate deepcopy functions for Volume
	${GO_ENV_VARS} go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.2

download-grpc-health-probe:
	curl -OJL http://asdrepo.isus.emc.com:8081/artifactory/ecs-build/com/github/grpc-ecosystem/grpc-health-probe/0.3.1/grpc_health_probe-linux-amd64
	chmod +x grpc_health_probe-linux-amd64
	mv grpc_health_probe-linux-amd64 build/health_probe

generate-deepcopy:
	# Generate deepcopy functions for Volume and AvailableCapacity
	controller-gen object paths=api/v1/volumecrd/volume_types.go paths=api/v1/volumecrd/groupversion_info.go  output:dir=api/v1/volumecrd
	controller-gen object paths=api/v1/availablecapacitycrd/availablecapacity_types.go paths=api/v1/availablecapacitycrd/groupversion_info.go  output:dir=api/v1/availablecapacitycrd
	controller-gen object paths=api/v1/drivecrd/drive_types.go paths=api/v1/drivecrd/groupversion_info.go  output:dir=api/v1/drivecrd
	controller-gen object paths=api/v1/lvgcrd/lvg_types.go paths=api/v1/lvgcrd/groupversion_info.go  output:dir=api/v1/lvgcrd

generate-crd:
    #Generate CRD based on Volume and AvailableCapacity type and group info
	controller-gen crd:trivialVersions=true paths=api/v1/availablecapacitycrd/availablecapacity_types.go paths=api/v1/availablecapacitycrd/groupversion_info.go output:crd:dir=charts/baremetal-csi-plugin/crds
	controller-gen crd:trivialVersions=true paths=api/v1/volumecrd/volume_types.go paths=api/v1/volumecrd/groupversion_info.go output:crd:dir=charts/baremetal-csi-plugin/crds
	controller-gen crd:trivialVersions=true paths=api/v1/drivecrd/drive_types.go paths=api/v1/drivecrd/groupversion_info.go output:crd:dir=charts/baremetal-csi-plugin/crds
	controller-gen crd:trivialVersions=true paths=api/v1/lvgcrd/lvg_types.go paths=api/v1/lvgcrd/groupversion_info.go output:crd:dir=charts/baremetal-csi-plugin/crds

generate-api: compile-proto generate-crd generate-deepcopy
