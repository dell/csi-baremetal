# variables related to build in DELL EMC internal infrastructure
HAL_VERSION      := 3.4.0.0-1835.b1a54fa
REGISTRY         := 10.244.120.194:8085/atlantic
# TODO: remove HARBOR at all, AK8S-426
HARBOR           := harbor.lss.emc.com/atlantic

HEALTH_PROBE_BIN_URL := http://asdrepo.isus.emc.com:8081/artifactory/ecs-build/com/github/grpc-ecosystem/grpc-health-probe/0.3.1/grpc_health_probe-linux-amd64

# go evn related
GOPRIVATE_PART	 := GOPRIVATE=eos2git.cec.lab.emc.com/*
# override variable in variables.mk
GOPROXY_PART     := GOPROXY=http://asdrepo.isus.emc.com/artifactory/api/go/ecs-go-build,https://proxy.golang.org,direct
