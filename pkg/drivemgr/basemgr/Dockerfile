FROM    basemgr:base

LABEL   description="Bare-metal CSI Base Drive Manager"

ADD     basemgr base-drivemgr

EXPOSE  8888

RUN groupadd -r csiuser && useradd -r -g csiuser csiuser
USER csiuser

ENTRYPOINT  ["./base-drivemgr"]
