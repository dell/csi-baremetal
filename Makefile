# build command:
# REGISTRY=<registry_url> TAG="<tag_name>" make all

include variables.mk

NODE         := node
HW_MANAGER   := hwmgr
CONTROLLER   := controller
HEALTH_PROBE := health_probe

.PHONY: test build install-hal

#all: build image push

# use in clear environment
prepare-env: install-compile-proto install-hal install-controller-gen dependency generate-api

dependency:
	GO111MODULE=on go mod download

build: build-hwmgr build-node build-controller

# NOTE: Output directory for binary file should be in Docker context.
# So we can't use /baremetal-csi-plugin/build to build the image.
build-hwmgr:
	go build -o ./build/${HW_MANAGER}/${HW_MANAGER} ./cmd/${HW_MANAGER}/main.go

build-node:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${NODE}/${NODE} ./cmd/${NODE}/main.go

build-controller:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${CONTROLLER}/${CONTROLLER} ./cmd/${CONTROLLER}/main.go

image: image-hwmgr image-node image-controller

image-hwmgr:
	cp ./build/${HW_MANAGER}/${HW_MANAGER} ./pkg/${HW_MANAGER}/${HW_MANAGER}
	docker build --network host --force-rm --tag ${REGISTRY}/${REPO}-${HW_MANAGER}:${TAG} ./pkg/${HW_MANAGER}
	docker tag ${REGISTRY}/${REPO}-${HW_MANAGER}:${TAG} ${HARBOR}/${REPO}-${HW_MANAGER}:${TAG}

image-node: download-grpc-health-probe
	cp ./build/${NODE}/${NODE} ./pkg/${NODE}/${NODE}
	cp ./build/${HEALTH_PROBE} ./pkg/${NODE}/${HEALTH_PROBE}
	docker build --network host --force-rm --tag ${REGISTRY}/${REPO}-${NODE}:${TAG} ./pkg/${NODE}
	docker tag ${REGISTRY}/${REPO}-${NODE}:${TAG} ${HARBOR}/${REPO}-${NODE}:${TAG}

image-controller:
	cp ./build/${CONTROLLER}/${CONTROLLER} ./pkg/${CONTROLLER}/${CONTROLLER}
	docker build --network host --force-rm --tag ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG} ./pkg/${CONTROLLER}
	docker tag ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG} ${HARBOR}/${REPO}-${CONTROLLER}:${TAG}

push: push-hwmgr push-node push-controller

push-local:
	docker push ${REGISTRY}/${REPO}-${HW_MANAGER}:${TAG}
	docker push ${REGISTRY}/${REPO}-${NODE}:${TAG}
	docker push ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG}

push-hwmgr:
	docker push ${REGISTRY}/${REPO}-${HW_MANAGER}:${TAG}
	docker push ${HARBOR}/${REPO}-${HW_MANAGER}:${TAG}

push-node:
	docker push ${REGISTRY}/${REPO}-${NODE}:${TAG}
	docker push ${HARBOR}/${REPO}-${NODE}:${TAG}

push-controller:
	docker push ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG}
	docker push ${HARBOR}/${REPO}-${CONTROLLER}:${TAG}\

clean: clean-hwmgr clean-node clean-controller

clean-hwmgr:
	rm -rf ./build/${HW_MANAGER}/${HW_MANAGER}

clean-node:
	rm -rf ./build/${NODE}/${NODE}

clean-controller:
	rm -rf ./build/${CONTROLLER}/${CONTROLLER}

clean-image: clean-image-hwmgr clean-image-node # clean-image-controller

clean-image-hwmgr:
	docker rmi ${REGISTRY}/${REPO}-${HW_MANAGER}:${TAG}

clean-image-node:
	docker rmi ${REGISTRY}/${REPO}-${NODE}:${TAG}

clean-image-controller:
	docker rmi ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG}

lint:
	golangci-lint run ./...

lint-charts:
	helm lint ./${CHARTS_PATH}

test:
	go test -race -cover ./... -coverprofile=coverage.out

coverage:
	go tool cover -html=coverage.out -o coverage.html

prepare-protoc:  # TODO: temporary solution while we build all in common devkit (DELIVERY-1488)
	mkdir -p proto_3.11.0
	curl -L -O https://github.com/protocolbuffers/protobuf/releases/download/v3.11.0/protoc-3.11.0-linux-x86_64.zip && \
	unzip protoc-3.11.0-linux-x86_64.zip -d proto_3.11.0/ && \
	sudo mv proto_3.11.0/bin/protoc /usr/bin/protoc && \
	protoc --version; rm -rf proto_3.11.0; rm protoc-*
	go get -u github.com/golang/protobuf/protoc-gen-go

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
	GO111MODULE=on go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.2

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
