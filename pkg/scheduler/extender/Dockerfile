FROM    alpine:3.16

LABEL   description="Bare-metal CSI Scheduler Extender"

ADD     extender  extender

ADD     health_probe    health_probe

RUN addgroup -S bmcsi && adduser -S bmcsi -u 1000 -G bmcsi

USER 1000

ENTRYPOINT ["/extender"]
