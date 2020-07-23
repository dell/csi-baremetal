FROM    ubuntu:20.04

LABEL   description="CSI-Baremetal Scheduler extender"

ADD     build/scheduler-extender  scheduler-extender

EXPOSE  8888

ENTRYPOINT ["/scheduler-extender"]
