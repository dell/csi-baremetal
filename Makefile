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

tidy:
	${GO_ENV_VARS} go mod tidy
	${GO_ENV_VARS} go get sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VER)
	${GO_ENV_VARS} go get github.com/vektra/mockery/v2@$(MOCKERY_VER)

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
	GOOS=linux go build -o ./build/${DRIVE_MANAGER}/$(DRIVE_MANAGER_TYPE)/$(DRIVE_MANAGER_TYPE) ${LDFLAGS} ./cmd/${DRIVE_MANAGER}/$(DRIVE_MANAGER_TYPE)/main.go

build-node:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${NODE}/${NODE} ${LDFLAGS} ./cmd/${NODE}/main.go

build-controller:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${CONTROLLER}/${CONTROLLER} ${LDFLAGS} ./cmd/${CONTROLLER}/main.go

build-extender:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${SCHEDULING_PKG}/${EXTENDER}/${EXTENDER} ${LDFLAGS} ./cmd/${SCHEDULING_PKG}/${EXTENDER}/main.go

build-scheduler:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${SCHEDULING_PKG}/${SCHEDULER}/${SCHEDULER} ${LDFLAGS} ./cmd/${SCHEDULING_PKG}/${SCHEDULER}/main.go

build-node-controller:
	CGO_ENABLED=0 GOOS=linux go build -o ./build/${CR_CONTROLLERS}/${NODE_CONTROLLER}/${CONTROLLER} ${LDFLAGS} ./cmd/${NODE_CONTROLLER}/main.go

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

clean-smart:
	rm -rf ./api/smart/generated/*

### API targets
install-protoc:
	mkdir -p proto_3.11.0
	curl -L -O https://github.com/protocolbuffers/protobuf/releases/download/v3.11.0/protoc-3.11.0-linux-x86_64.zip && \
	unzip protoc-3.11.0-linux-x86_64.zip -d proto_3.11.0/ && \
	sudo mv proto_3.11.0/bin/protoc /usr/bin/protoc && \
	protoc --version; rm -rf proto_3.11.0; rm protoc-*
	# TODO update to google.golang.org/protobuf - https://github.com/dell/csi-baremetal/issues/613
	go install github.com/golang/protobuf/protoc-gen-go@$(PROTOC_GEN_GO_VER)

install-compile-proto: install-protoc compile-proto

install-controller-gen:
	# Instgall controller-gen
	${GO_ENV_VARS} go install sigs.k8s.io/controller-tools/cmd/controller-gen@${CONTROLLER_GEN_VER}
	${GO_ENV_VARS} go mod download go.uber.org/goleak

install-mockery:
    # Install mockery
	${GO_ENV_VARS} go install github.com/vektra/mockery/v2@${MOCKERY_VER}

compile-proto:
	mkdir -p api/generated/v1/
	protoc -I=api/v1 --go_out=plugins=grpc:api/generated/v1 api/v1/*.proto

generate-deepcopy:
	# Generate deepcopy functions for CRD
	controller-gen object paths=api/v1/volumecrd/volume_types.go paths=api/v1/volumecrd/groupversion_info.go  output:dir=api/v1/volumecrd
	controller-gen object paths=api/v1/availablecapacitycrd/availablecapacity_types.go paths=api/v1/availablecapacitycrd/groupversion_info.go  output:dir=api/v1/availablecapacitycrd
	controller-gen object paths=api/v1/acreservationcrd/availablecapacityreservation_types.go paths=api/v1/acreservationcrd/groupversion_info.go  output:dir=api/v1/acreservationcrd
	controller-gen object paths=api/v1/drivecrd/drive_types.go paths=api/v1/drivecrd/groupversion_info.go  output:dir=api/v1/drivecrd
	controller-gen object paths=api/v1/lvgcrd/logicalvolumegroup_types.go paths=api/v1/lvgcrd/groupversion_info.go  output:dir=api/v1/lvgcrd
	controller-gen object paths=api/v1/nodecrd/node_types.go paths=api/v1/nodecrd/groupversion_info.go  output:dir=api/v1/nodecrd
	controller-gen object paths=api/v1/storagegroupcrd/storagegroup_types.go paths=api/v1/storagegroupcrd/groupversion_info.go  output:dir=api/v1/storagegroupcrd

generate-baremetal-crds: install-controller-gen
	controller-gen $(CRD_OPTIONS) paths=api/v1/availablecapacitycrd/availablecapacity_types.go paths=api/v1/availablecapacitycrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	controller-gen $(CRD_OPTIONS) paths=api/v1/acreservationcrd/availablecapacityreservation_types.go paths=api/v1/acreservationcrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	controller-gen $(CRD_OPTIONS) paths=api/v1/volumecrd/volume_types.go paths=api/v1/volumecrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	controller-gen $(CRD_OPTIONS) paths=api/v1/drivecrd/drive_types.go paths=api/v1/drivecrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	controller-gen $(CRD_OPTIONS) paths=api/v1/lvgcrd/logicalvolumegroup_types.go paths=api/v1/lvgcrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	controller-gen $(CRD_OPTIONS) paths=api/v1/nodecrd/node_types.go paths=api/v1/nodecrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)
	controller-gen $(CRD_OPTIONS) paths=api/v1/storagegroupcrd/storagegroup_types.go paths=api/v1/storagegroupcrd/groupversion_info.go output:crd:dir=$(CSI_CHART_CRDS_PATH)

generate-smart:
	go generate ./api/smart/...

generate-api: compile-proto generate-baremetal-crds generate-deepcopy generate-smart

# Used for UT. Need to regenerate after updating k8s API version
generate-mocks: install-mockery
	mockery --dir=/usr/local/go/pkg/mod/k8s.io/client-go\@$(CLIENT_GO_VER)/kubernetes/typed/core/v1/ --name=EventInterface --output=pkg/events/mocks

run-csi-baremetal-functional-tests:
	@echo "Configuring functional tests for csi baremetal..."; \
	pwd; \
	ls -la; \
	sed -i '/parser.addoption("--login", action="store", default=""/s/default=""/default="${USERNAME}"/' ${PROJECT_DIR}/tests/e2e-test-framework/conftest.py; \
	sed -i '/parser.addoption("--password", action="store", default=""/s/default=""/default="${PASSWORD}"/' ${PROJECT_DIR}/tests/e2e/conftest.py; \
	sed -i '/parser.addoption("--qtest_token", action="store", default=""/s/default=""/default="${QTEST_API_KEY}"/' ${PROJECT_DIR}/tests/e2e/conftest.py; \
	sed -i '/parser.addoption("--qtest_test_suite", action="store", default=""/s/default=""/default="${QTEST_SUITE_ID}"/' ${PROJECT_DIR}/tests/e2e/conftest.py; \
	echo "conftest.py:"; \
	cat ${PROJECT_DIR}/tests/e2e-test-framework/conftest.py; \
	echo "Copying test files to remote server..."; \
	sshpass -p '${PASSWORD}' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null ${USERNAME}@${E2E_VM_SERVICE_NODE_IP} "mkdir -p /root/tests/csi-baremetal-e2e"; \
	sshpass -p '${PASSWORD}' scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -r ${PROJECT_DIR}/tests/e2e ${USERNAME}@${E2E_VM_SERVICE_NODE_IP}:/root/tests/csi-baremetal-e2e; \
	echo "Installing dependencies and running tests on remote server..."; \
	sshpass -p '${PASSWORD}' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null ${USERNAME}@${E2E_VM_SERVICE_NODE_IP} '. /root/venv/python3.12.2/bin/activate && ls -la && pip3 install -r requirements.txt && pytest --junitxml=test_results_csi_baremetal.xml ${TEST_FILTER_CSI_BAREMETAL}'; \
	echo "Copying test results back to local machine..."; \
	sshpass -p '${PASSWORD}' scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -r ${USERNAME}@${E2E_VM_SERVICE_NODE_IP}:/root/tests/csi-baremetal-e2e/e2e-test-framework/test_results_csi_baremetal.xml ${PROJECT_DIR}/test_results_csi_baremetal.xml; \
	TEST_EXIT_CODE=$$?; \
	echo "Test exit code: $$TEST_EXIT_CODE"; \
	if [ -e "${PROJECT_DIR}/test_results_csi_baremetal.xml" ]; then \
		echo "Test results for csi-baremetal copied successfully."; \
	else \
		echo "Error: Failed to copy test results for csi-baremetal."; \
		exit 1; \
	fi; \
	if [ $$TEST_EXIT_CODE -eq 0 ]; then \
		echo "All tests for csi-baremetal passed successfully."; \
	else \
		echo "Functional tests for csi-baremetal failed."; \
		exit 1; \
	fi;

#cleanup test files on remote server
functional-tests-cleanup:
	@echo "Cleaning up functional test files on remote server..."; \
	sshpass -p '${PASSWORD}' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null ${USERNAME}@${E2E_VM_SERVICE_NODE_IP} 'rm -rf /root/tests/*'; \
	echo "Functional test cleanup completed."

.PHONY: csi-baremetal-functional-tests
csi-baremetal-functional-tests: \
	functional-tests-cleanup \
	run-csi-baremetal-functional-tests \
	functional-tests-cleanup
	@echo "Functional tests for csi-baremetal completed successfully."