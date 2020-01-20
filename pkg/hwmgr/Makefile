all: build

HAL_VERSION="3.5.0.0-1821.cd52a26"

build: install-hal
	# NOTE: Output directory for binary file should be in Docker context. So we can't use /baremetal-csi-plugin/cmd
	go build -o ./../../cmd/hwmgr/_output/hw-manager ./../../cmd/hwmgr/main.go

.PNONY: install-hal
install-hal:
	# NOTE: Root privileges are required for installing or uninstalling packages.
	sudo zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends http://asdrepo.isus.emc.com:8081/artifactory/ecs-prerelease-local/com/emc/asd/vipr/sles12/viprhal/viprhal/${HAL_VERSION}-go.SLES/viprhal-${HAL_VERSION}.SLES.x86_64.rpm
	sudo zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends http://asdrepo.isus.emc.com:8081/artifactory/ecs-prerelease-local/com/emc/asd/vipr/sles12/viprhal/viprhal-devel/${HAL_VERSION}-go.SLES/viprhal-devel-${HAL_VERSION}.SLES.x86_64.rpm
