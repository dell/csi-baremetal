include variables.mk

include Makefile.docker
include Makefile.validation
# optional include
-include Makefile.addition

.PHONY: version test build

# print version
version:
	@printf $(TAG)

dependency:
	${GO_ENV_VARS} go mod download

all: build base-images images push

### Build binaries
build: \
build-drivemgr \
build-node \
build-controller \
build-extender \
build-scheduler \
build-node-controller

build-drivemgr:
	GOOS=linux go build -o ./build/${DRIVE_MANAGER}/$(DRIVE_MANAGER_TYPE)/$(DRIVE_MANAGER_TYPE) ./cmd/${DRIVE_MANAGER}/$(DRIVE_MANAGER_TYPE)/main.go

build-node:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${NODE}/${NODE} ${LDFLAGS} ./cmd/${NODE}/main.go

build-controller:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${CONTROLLER}/${CONTROLLER} ${LDFLAGS} ./cmd/${CONTROLLER}/main.go

build-extender:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${SCHEDULING_PKG}/${EXTENDER}/${EXTENDER} ./cmd/${SCHEDULING_PKG}/${EXTENDER}/main.go

build-scheduler:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${SCHEDULING_PKG}/${SCHEDULER}/${SCHEDULER} ./cmd/${SCHEDULING_PKG}/${SCHEDULER}/main.go

build-node-controller:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${CR_CONTROLLERS}/${NODE_CONTROLLER}/${CONTROLLER} ./cmd/${NODE_CONTROLLER}/main.go

### Clean artifacts
clean-all: clean clean-images

clean: clean-drivemgr \
clean-node \
clean-controller \
clean-extender \
clean-scheduler \
clean-node-controller

clean-drivemgr:
	rm -rf ./build/${DRIVE_MANAGER}/*

clean-node:
	rm -rf ./build/${NODE}/*

clean-controller:
	rm -rf ./build/${CONTROLLER}/*

clean-extender:
	rm -rf ./build/${SCHEDULING_PKG}/${EXTENDER}/*

clean-scheduler:
	rm -rf ./build/${SCHEDULING_PKG}/${SCHEDULER}/*

clean-node-controller:
	rm -rf ./build/${CR_CONTROLLERS}/*

clean-proto:
	rm -rf ./api/generated/v1/*

### API targets
install-protoc:
	mkdir -p proto_3.11.0
	curl -L -O https://github.com/protocolbuffers/protobuf/releases/download/v3.11.0/protoc-3.11.0-linux-x86_64.zip && \
	unzip protoc-3.11.0-linux-x86_64.zip -d proto_3.11.0/ && \
	sudo mv proto_3.11.0/bin/protoc /usr/bin/protoc && \
	protoc --version; rm -rf proto_3.11.0; rm protoc-*
	go get -u github.com/golang/protobuf/protoc-gen-go@v1.3.2

install-compile-proto: install-protoc compile-proto

install-controller-gen:
	# Build controller-gen from go.mod
	${GO_ENV_VARS} go mod download sigs.k8s.io/controller-tools
	${GO_ENV_VARS} go build -mod='' -o ./bin/ sigs.k8s.io/controller-tools/cmd/controller-gen

compile-proto:
	mkdir -p api/generated/v1/
	protoc -I=api/v1 --go_out=plugins=grpc:api/generated/v1 api/v1/*.proto

generate-deepcopy:
	# Generate deepcopy functions for CRD
	$(CONTROLLER_GEN_BIN) object paths=api/v1/volumecrd/volume_types.go paths=api/v1/volumecrd/groupversion_info.go  output:dir=api/v1/volumecrd
	$(CONTROLLER_GEN_BIN) object paths=api/v1/availablecapacitycrd/availablecapacity_types.go paths=api/v1/availablecapacitycrd/groupversion_info.go  output:dir=api/v1/availablecapacitycrd
	$(CONTROLLER_GEN_BIN) object paths=api/v1/acreservationcrd/availablecapacityreservation_types.go paths=api/v1/acreservationcrd/groupversion_info.go  output:dir=api/v1/acreservationcrd
	$(CONTROLLER_GEN_BIN) object paths=api/v1/drivecrd/drive_types.go paths=api/v1/drivecrd/groupversion_info.go  output:dir=api/v1/drivecrd
	$(CONTROLLER_GEN_BIN) object paths=api/v1/lvgcrd/logicalvolumegroup_types.go paths=api/v1/lvgcrd/groupversion_info.go  output:dir=api/v1/lvgcrd
	$(CONTROLLER_GEN_BIN) object paths=api/v1/nodecrd/node_types.go paths=api/v1/nodecrd/groupversion_info.go  output:dir=api/v1/nodecrd

generate-baremetal-crds:
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=api/v1/availablecapacitycrd/availablecapacity_types.go paths=api/v1/availablecapacitycrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=api/v1/acreservationcrd/availablecapacityreservation_types.go paths=api/v1/acreservationcrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=api/v1/volumecrd/volume_types.go paths=api/v1/volumecrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=api/v1/drivecrd/drive_types.go paths=api/v1/drivecrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=api/v1/lvgcrd/logicalvolumegroup_types.go paths=api/v1/lvgcrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	$(CONTROLLER_GEN_BIN) $(CRD_OPTIONS) paths=api/v1/nodecrd/node_types.go paths=api/v1/nodecrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)

generate-api: compile-proto generate-crds generate-deepcopy
