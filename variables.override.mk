# variables related to build in DELL EMC internal infrastructure
HAL_VERSION      := 3.4.0.0-1835.b1a54fa
REGISTRY         := asdrepo.isus.emc.com:9042

HEALTH_PROBE_BIN_URL := http://asdrepo.isus.emc.com:8081/artifactory/ecs-build/com/github/grpc-ecosystem/grpc-health-probe/0.3.1/grpc_health_probe-linux-amd64

# go evn related
GOPRIVATE_PART	 := GOPRIVATE=eos2git.cec.lab.emc.com/*
# override variable in variables.mk
GOPROXY_PART     := GOPROXY=http://asdrepo.isus.emc.com/artifactory/api/go/ecs-go-build,https://proxy.golang.org,direct
