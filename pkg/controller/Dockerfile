FROM    controller:base

LABEL   description="Bare-metal CSI Controller Service"

ADD     controller /controller

RUN groupadd -r csiuser && useradd -r -g csiuser csiuser
USER csiuser

ENTRYPOINT ["/controller"]
