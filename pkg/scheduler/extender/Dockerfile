FROM    alpine:3.14

LABEL   description="Bare-metal CSI Scheduler Extender"

ADD     extender  extender

ADD     health_probe    health_probe

ENTRYPOINT ["/extender"]
