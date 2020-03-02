FROM    ubuntu:18.04

LABEL   description="Bare-metal CSI Node Service"

ADD     node            node
ADD     health_probe    health_probe

RUN     apt update --no-install-recommends -y -q; apt install --no-install-recommends -y -q curl util-linux parted xfsprogs lvm2 gdisk strace udev net-tools

EXPOSE  9999

ENTRYPOINT ["/node"]
