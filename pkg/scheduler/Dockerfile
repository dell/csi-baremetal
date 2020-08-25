FROM    ubuntu:20.04

LABEL   description="CSI-Baremetal Scheduler extender"

ADD     extender  extender
ADD		schedulerpatcher  schedulerpatcher

ENTRYPOINT ["/extender"]
