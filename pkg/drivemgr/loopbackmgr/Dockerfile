FROM    loopbackmgr:base

LABEL   description="Bare-metal CSI Loopback Drive Manager"

ADD     loopbackmgr loopback-drivemgr

EXPOSE  8888

RUN useradd -r -g sudo csiuser
USER csiuser

ENTRYPOINT  ["./loopback-drivemgr"]
