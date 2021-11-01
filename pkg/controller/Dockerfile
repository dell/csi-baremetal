FROM    controller:base

LABEL   description="Bare-metal CSI Controller Service"

ADD     controller /controller

RUN addgroup -S bmcsi && adduser -S bmcsi -G bmcsi

USER bmcsi

ENTRYPOINT ["/controller"]
