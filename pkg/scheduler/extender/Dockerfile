FROM    alpine:3.14

LABEL   description="Bare-metal CSI Scheduler Extender"

ADD     extender  extender

ADD     health_probe    health_probe

RUN addgroup -S csiuser && adduser -S csiuser -G csiuser
USER csiuser

ENTRYPOINT ["/extender"]
