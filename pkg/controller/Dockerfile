FROM    controller:base

LABEL   description="Bare-metal CSI Controller Service"

ADD     controller /controller

RUN addgroup -S csiuser && adduser -S csiuser -G csiuser
USER csiuser

ENTRYPOINT ["/controller"]
