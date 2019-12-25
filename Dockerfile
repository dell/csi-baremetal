FROM alpine:3.10

LABEL description="Bare-metal CSI Driver"

COPY ./build/_output/baremetal_csi  /baremetal_csi

RUN  apk add curl util-linux parted xfsprogs-extra lvm2

ENTRYPOINT ["/baremetal_csi"]
