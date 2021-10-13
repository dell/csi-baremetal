FROM    alpine:3.14

LABEL   description="Bare-metal CSI Scheduler"

ADD     scheduler  scheduler 

RUN addgroup -S csiuser && adduser -S csiuser -G csiuser
USER csiuser

ENTRYPOINT ["/scheduler"]
