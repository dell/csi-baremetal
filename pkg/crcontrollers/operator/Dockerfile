FROM    ubuntu:20.04

LABEL   description="Baremetal CSI Operator"

COPY csi-baremetal-driver /csi-baremetal-driver
ADD     operator  csi-operator

ENTRYPOINT ["/csi-operator"]
