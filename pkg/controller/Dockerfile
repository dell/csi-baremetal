FROM alpine:3.10

LABEL   description="Bare-metal CSI Node Service"

ADD     controller controller

RUN     apk add curl util-linux parted xfsprogs-extra lvm2 sgdisk

ENTRYPOINT ["/controller"]
