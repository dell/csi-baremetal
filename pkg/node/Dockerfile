FROM    node:base

LABEL   description="Bare-metal CSI Node Service"

ADD     node  node

EXPOSE  9999

RUN sed -i \
        -e 's/udev_sync = 1/udev_sync = 0/' \
        -e 's/udev_rules = 1/udev_rules = 0/' \
        -e 's/obtain_device_list_from_udev = 1/obtain_device_list_from_udev = 0/' \
        /etc/lvm/lvm.conf

ENTRYPOINT ["/node"]
