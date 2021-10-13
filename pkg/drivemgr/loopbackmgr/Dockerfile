FROM    loopbackmgr:base

LABEL   description="Bare-metal CSI Loopback Drive Manager"

ADD     loopbackmgr loopback-drivemgr

EXPOSE  8888

RUN groupadd -r csiuser && useradd -r -g csiuser csiuser
USER csiuser

ENTRYPOINT  ["./loopback-drivemgr"]
