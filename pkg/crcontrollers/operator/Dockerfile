FROM    ubuntu:20.04

LABEL   description="Baremetal CSI Operator"

ENV BASE_URL="https://get.helm.sh"
ENV TAR_FILE="helm-v3.0.0-linux-amd64.tar.gz"

# TODO remove this code when helm won't be required
RUN apt update --no-install-recommends -y -q && \
    apt install --no-install-recommends -y -q curl ca-certificates && \
    curl -L ${BASE_URL}/${TAR_FILE} |tar xvz && \
    mv linux-amd64/helm /usr/bin/helm && \
    chmod +x /usr/bin/helm && \
    rm -rf linux-amd64 && \
    rm -f /var/cache/apk/*

COPY csi-baremetal-driver /csi-baremetal-driver
ADD     operator  csi-operator

ENTRYPOINT ["/csi-operator"]
