FROM    alpine:3.14

LABEL   description="Baremetal CSI Operator"

ADD     controller  node-controller

RUN addgroup -S bmcsi && adduser -S bmcsi -G bmcsi

USER bmcsi

ENTRYPOINT ["/node-controller"]
