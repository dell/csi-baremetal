FROM    drivemgr:base

LABEL   description="Bare-metal CSI Drive Manager Service"

# TODO: do not copy all binaries: AK8S-1125
COPY    hal-drivemgr  idrac-drivemgr  loopback-drivemgr entrypoint.sh ./

EXPOSE  8888

ENTRYPOINT ["/entrypoint.sh"]
