FROM    ubuntu:20.04

LABEL   description="CSI-Baremetal Scheduler plugin"

ADD     scheduler  scheduler
ADD     scheduler-config.yaml /etc/kubernetes/csi-baremetal-scheduler.conf

ENTRYPOINT ["/scheduler"]
