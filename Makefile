# build command:
# REGISTRY=<registry_url> TAG="<tag_name>" make all

include variables.mk

NODE       := node
HW_MANAGER := hwmgr
CONTROLLER := controller

.PHONY: test build install-hal

#all: build image push

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

image: image-hwmgr image-node # image-controller

image-hwmgr:
	cp ./build/${HW_MANAGER}/${HW_MANAGER} ./pkg/${HW_MANAGER}/${HW_MANAGER}
	docker build --network host --force-rm --tag ${REGISTRY}/${REPO}-${HW_MANAGER}:${TAG} ./pkg/${HW_MANAGER}
	docker tag ${REGISTRY}/${REPO}-${HW_MANAGER}:${TAG} ${HARBOR}/${REPO}-${HW_MANAGER}:${TAG}

image-node:
	cp ./build/${NODE}/${NODE} ./pkg/${NODE}/${NODE}
	docker build --network host --force-rm --tag ${REGISTRY}/${REPO}-${NODE}:${TAG} ./pkg/${NODE}
	docker tag ${REGISTRY}/${REPO}-${NODE}:${TAG} ${HARBOR}/${REPO}-${NODE}:${TAG}

image-controller:
	cp ./build/${CONTROLLER}/${CONTROLLER} ./pkg/${CONTROLLER}/${CONTROLLER}
	docker build --network host --force-rm --tag ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG} ./pkg/${CONTROLLER}
	docker tag ${REGISTRY}/${REPO}-${CONTROLLER}:${TAG} ${HARBOR}/${REPO}-${CONTROLLER}:${TAG}

push: push-hwmgr push-node # push-controller

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
	sudo zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends http://asdrepo.isus.emc.com:8081/artifactory/ecs-prerelease-local/com/emc/asd/vipr/sles12/viprhal/viprhal/${HAL_VERSION}-go.SLES/viprhal-${HAL_VERSION}.SLES.x86_64.rpm
	sudo zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends http://asdrepo.isus.emc.com:8081/artifactory/ecs-prerelease-local/com/emc/asd/vipr/sles12/viprhal/viprhal-devel/${HAL_VERSION}-go.SLES/viprhal-devel-${HAL_VERSION}.SLES.x86_64.rpm

install-controller-gen:
	# Generate deepcopy functions for Volume
	GO111MODULE=on go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.2

generate-deepcopy:
	# Generate deepcopy functions for Volume
	controller-gen object paths=api/v1/volume_types.go  output:dir=api/v1

generate-crd:
	# Generate CRD based on Volume type and group info
	controller-gen crd:crdVersions=v1beta1,trivialVersions=true paths=api/v1/volume_types.go paths=api/v1/groupversion_info.go  output:crd:dir=charts/baremetal-csi-plugin/templates
