FROM    alpine:3.14

LABEL   description="Baremetal CSI Operator"

ADD     controller  node-controller

RUN groupadd -r csiuser && useradd -r -g csiuser csiuser
USER csiuser

ENTRYPOINT ["/node-controller"]
