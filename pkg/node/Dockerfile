FROM alpine:3.10

LABEL   description="Bare-metal CSI Node Service"

ADD     node node

RUN     apk add curl util-linux parted xfsprogs-extra lvm2 sgdisk

EXPOSE  50051

ENTRYPOINT ["/node"]

