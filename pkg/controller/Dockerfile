FROM    controller:base

LABEL   description="Bare-metal CSI Controller Service"

ADD     controller /controller

RUN addgroup -S bmcsi && adduser -S bmcsi -u 1000 -G bmcsi

USER 1000

EXPOSE 8787

ENTRYPOINT ["/controller", "-metrics-address=:8787", "-metrics-path=/metrics"]
