FROM    alpine:3.14

LABEL   description="Bare-metal CSI Scheduler"

ADD     scheduler  scheduler 

RUN groupadd -r csiuser && useradd -r -g csiuser csiuser
USER csiuser

ENTRYPOINT ["/scheduler"]
