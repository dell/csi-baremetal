FROM    alpine:3.14

LABEL   description="Baremetal CSI Operator"

ADD     controller  node-controller

RUN addgroup -S bmcsi && adduser -S bmcsi -u 1000 -G bmcsi

USER 1000

ENTRYPOINT ["/node-controller"]
