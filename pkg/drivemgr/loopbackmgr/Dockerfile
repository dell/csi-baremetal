FROM    loopbackmgr:base

LABEL   description="Bare-metal CSI Drive Manager Service based on loopback devices"

ADD     loopbackmgr loopback-drivemgr

EXPOSE  8888

ENTRYPOINT  ["./loopback-drivemgr"]
