include variables.mk

include Makefile.validation
# optional include
-include Makefile.addition

.PHONY: version test build

# print version
version:
	@printf $(TAG)

dependency:
	${GO_ENV_VARS} go mod download

### Build binaries

build: compile-proto build-drivemgr build-node build-controller

build-drivemgr:
ifeq ($(DRIVE_MANAGER_TYPE),)
	# build all drivemanagers: TODO: do not build all binaries, AK8S-1125
	go build -o ./build/${DRIVE_MANAGER}/hal-drivemgr ./cmd/${DRIVE_MANAGER}/halmgr/main.go
	go build -o ./build/${DRIVE_MANAGER}/loopback-drivemgr ./cmd/${DRIVE_MANAGER}/loopbackmgr/main.go
	go build -o ./build/${DRIVE_MANAGER}/idrac-drivemgr ./cmd/${DRIVE_MANAGER}/idracmgr/main.go
	go build -o ./build/${DRIVE_MANAGER}/base-drivemgr ./cmd/${DRIVE_MANAGER}/basemgr/main.go
endif
ifneq ($(DRIVE_MANAGER_TYPE),)
	go build -o ./build/${DRIVE_MANAGER}/${DRIVE_MANAGER} ./cmd/${DRIVE_MANAGER}/${DRIVE_MANAGER_TYPE}/main.go
endif

build-node:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${NODE}/${NODE} ./cmd/${NODE}/main.go

build-controller:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${CONTROLLER}/${CONTROLLER} ./cmd/${CONTROLLER}/main.go

### Build images

images: image-drivemgr image-node image-controller

base-images: base-image-drivemgr base-image-node base-image-controller

base-image-drivemgr:
ifeq ($(DRIVE_MANAGER_TYPE), basemgr)
	docker build --network host --force-rm --file ./pkg/${DRIVE_MANAGER}/$(DRIVE_MANAGER_TYPE)/Dockerfile.build --tag ${DRIVE_MANAGER_TYPE}:base ./pkg/${DRIVE_MANAGER}/$(DRIVE_MANAGER_TYPE)/
else
	docker build --network host --force-rm --file ./pkg/${DRIVE_MANAGER}/Dockerfile.build --tag ${DRIVE_MANAGER}:base ./pkg/${DRIVE_MANAGER}
endif

download-grpc-health-probe:
	curl -OJL ${HEALTH_PROBE_BIN_URL}
	chmod +x grpc_health_probe-linux-amd64
	mv grpc_health_probe-linux-amd64 build/health_probe

# NOTE: Output directory for binary file should be in Docker context.
# So we can't use /baremetal-csi-plugin/build to build the image.
base-image-node: download-grpc-health-probe
	cp ./build/${HEALTH_PROBE} ./pkg/${NODE}/${HEALTH_PROBE}
	docker build --network host --force-rm --file ./pkg/${NODE}/Dockerfile.build --tag ${NODE}:base ./pkg/${NODE}

base-image-controller:
	docker build --network host --force-rm --file ./pkg/${CONTROLLER}/Dockerfile.build --tag ${CONTROLLER}:base ./pkg/${CONTROLLER}

image-drivemgr:
ifeq ($(DRIVE_MANAGER_TYPE), basemgr)
	cp ./build/${DRIVE_MANAGER}/${DRIVE_MANAGER} ./pkg/${DRIVE_MANAGER}/$(DRIVE_MANAGER_TYPE)/
	docker build --network host --force-rm --tag ${REGISTRY}/${PROJECT}-${DRIVE_MANAGER}:${TAG} ./pkg/${DRIVE_MANAGER}/$(DRIVE_MANAGER_TYPE)/
else
	cp ./build/${DRIVE_MANAGER}/* ./pkg/${DRIVE_MANAGER}/
	docker build --network host --force-rm --tag ${REGISTRY}/${PROJECT}-${DRIVE_MANAGER}:${TAG} ./pkg/${DRIVE_MANAGER}
endif

image-node:
	cp ./build/${NODE}/${NODE} ./pkg/${NODE}/${NODE}
	docker build --network host --force-rm --tag ${REGISTRY}/${PROJECT}-${NODE}:${TAG} ./pkg/${NODE}

image-controller:
	cp ./build/${CONTROLLER}/${CONTROLLER} ./pkg/${CONTROLLER}/${CONTROLLER}
	docker build --network host --force-rm --tag ${REGISTRY}/${PROJECT}-${CONTROLLER}:${TAG} ./pkg/${CONTROLLER}

### Push images

push: push-drivemgr push-node push-controller

push-local:
	docker push ${REGISTRY}/${PROJECT}-${DRIVE_MANAGER}:${TAG}
	docker push ${REGISTRY}/${PROJECT}-${NODE}:${TAG}
	docker push ${REGISTRY}/${PROJECT}-${CONTROLLER}:${TAG}

push-drivemgr:
	docker push ${REGISTRY}/${PROJECT}-${DRIVE_MANAGER}:${TAG}

push-node:
	docker push ${REGISTRY}/${PROJECT}-${NODE}:${TAG}

push-controller:
	docker push ${REGISTRY}/${PROJECT}-${CONTROLLER}:${TAG}

### Clean artefacts

clean: clean-drivemgr clean-node clean-controller clean-proto

clean-drivemgr:
	rm -rf ./build/${DRIVE_MANAGER}/*

clean-node:
	rm -rf ./build/${NODE}/${NODE}

clean-controller:
	rm -rf ./build/${CONTROLLER}/${CONTROLLER}

clean-proto:
	rm -rf ./api/generated/v1/*

clean-images: clean-image-drivemgr clean-image-node clean-image-controller

clean-image-drivemgr:
	docker rmi ${REGISTRY}/${PROJECT}-${DRIVE_MANAGER}:${TAG}

clean-image-node:
	docker rmi ${REGISTRY}/${PROJECT}-${NODE}:${TAG}

clean-image-controller:
	docker rmi ${REGISTRY}/${PROJECT}-${CONTROLLER}:${TAG}

### API targets

install-protoc:
	mkdir -p proto_3.11.0
	curl -L -O https://github.com/protocolbuffers/protobuf/releases/download/v3.11.0/protoc-3.11.0-linux-x86_64.zip && \
	unzip protoc-3.11.0-linux-x86_64.zip -d proto_3.11.0/ && \
	sudo mv proto_3.11.0/bin/protoc /usr/bin/protoc && \
	protoc --version; rm -rf proto_3.11.0; rm protoc-*
	go get -u github.com/golang/protobuf/protoc-gen-go@v1.3.5

install-compile-proto: install-protoc compile-proto

install-controller-gen:
	# Generate deepcopy functions for Volume
	${GO_ENV_VARS} go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.2

compile-proto:
	mkdir -p api/generated/v1/
	protoc -I=api/v1 --go_out=plugins=grpc:api/generated/v1 api/v1/*.proto

generate-deepcopy:
	# Generate deepcopy functions for CRD
	controller-gen object paths=api/v1/volumecrd/volume_types.go paths=api/v1/volumecrd/groupversion_info.go  output:dir=api/v1/volumecrd
	controller-gen object paths=api/v1/availablecapacitycrd/availablecapacity_types.go paths=api/v1/availablecapacitycrd/groupversion_info.go  output:dir=api/v1/availablecapacitycrd
	controller-gen object paths=api/v1/drivecrd/drive_types.go paths=api/v1/drivecrd/groupversion_info.go  output:dir=api/v1/drivecrd
	controller-gen object paths=api/v1/lvgcrd/lvg_types.go paths=api/v1/lvgcrd/groupversion_info.go  output:dir=api/v1/lvgcrd

generate-crds:
    # Generate CRDs based on Volume and AvailableCapacity type and group info
	controller-gen crd:trivialVersions=true paths=api/v1/availablecapacitycrd/availablecapacity_types.go paths=api/v1/availablecapacitycrd/groupversion_info.go output:crd:dir=charts/baremetal-csi-plugin/crds
	controller-gen crd:trivialVersions=true paths=api/v1/volumecrd/volume_types.go paths=api/v1/volumecrd/groupversion_info.go output:crd:dir=charts/baremetal-csi-plugin/crds
	controller-gen crd:trivialVersions=true paths=api/v1/drivecrd/drive_types.go paths=api/v1/drivecrd/groupversion_info.go output:crd:dir=charts/baremetal-csi-plugin/crds
	controller-gen crd:trivialVersions=true paths=api/v1/lvgcrd/lvg_types.go paths=api/v1/lvgcrd/groupversion_info.go output:crd:dir=charts/baremetal-csi-plugin/crds

generate-api: compile-proto generate-crds generate-deepcopy
