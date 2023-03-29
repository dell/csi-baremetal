FROM    node:base

LABEL   description="Bare-metal CSI Node Service"

ADD     node  node

EXPOSE  9999 8787

ENTRYPOINT ["/node", "-metrics-address=:8787", "-metrics-path=/metrics"]
