FROM    node:base

LABEL   description="Bare-metal CSI Node Service"

ADD     node  node

EXPOSE  9999

RUN groupadd -r csiuser && useradd -r -g csiuser csiuser
USER csiuser

ENTRYPOINT ["/node"]
