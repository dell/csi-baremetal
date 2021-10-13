FROM    alpine:3.14

LABEL   description="Baremetal CSI Operator"

ADD     controller  node-controller

RUN addgroup -S csiuser && adduser -S csiuser -G csiuser
USER csiuser

ENTRYPOINT ["/node-controller"]
