FROM    ubuntu:20.04

LABEL   description="Bare-metal CSI Scheduler Extender"

ADD     extender  extender

ADD     health_probe    health_probe

ENTRYPOINT ["/extender"]
