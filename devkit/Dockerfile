# build kind binary
ARG KIND_BUILDER
FROM $KIND_BUILDER as builder

COPY          kind/ /kind/

RUN           apt update \
&&            apt install patch \
&&            chmod +x /kind/kind-build.sh \
&&            /kind/kind-build.sh /kind


FROM opensuse/leap:latest

ARG           arg_docker_ver
ARG           arg_go_ver
ARG           arg_golandci_ver
ARG           arg_kubectl_ver
ARG           arg_helm_ver
ARG           arg_protobuf_ver
ARG           arg_protoc_gen_go_ver
ARG           arg_python_ver

ENV           GOPATH="/usr/share/go"
ENV           GOROOT="/usr/local/go"
ENV           GOCACHE="$GOPATH/.cache/go-build"
ENV           GOENV="$GOPATH/.cache/go/env"
ENV           PROTOPATH="/usr/local/proto"
ENV           PATH="$PATH:$GOPATH/bin:$GOROOT/bin:$PROTOPATH/bin"


RUN           zypper --no-gpg-checks --non-interactive refresh \
&&            zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends \
              curl \
              docker-${arg_docker_ver} \
              gcc \
              git \
              jq \
              libXi6 \
              libXtst6 \
              make \
              ShellCheck \
              sudo \
              vim \
              wget \
              xorg-x11 \
              xorg-x11-fonts \
              unzip \
              MozillaFirefox \
              libasound2 \
              libgbm1 \
              libxshmfence1 \
              python3-devel-${arg_python_ver} \
              bash-completion
# install go
RUN           wget https://go.dev/dl/go${arg_go_ver}.linux-amd64.tar.gz \
&&            tar -C /usr/local -xzf go${arg_go_ver}.linux-amd64.tar.gz \
&&            rm go${arg_go_ver}.linux-amd64.tar.gz
# install bin golangci
RUN           curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin v${arg_golandci_ver}
# install proto
RUN 	      curl -L -O https://github.com/protocolbuffers/protobuf/releases/download/v${arg_protobuf_ver}/protoc-${arg_protobuf_ver}-linux-x86_64.zip \
&&    	      unzip protoc-${arg_protobuf_ver}-linux-x86_64.zip -d $PROTOPATH \
&&            rm protoc-${arg_protobuf_ver}-linux-x86_64.zip \
# TODO update to google.golang.org/protobuf - https://github.com/dell/csi-baremetal/issues/613
&&        	  go install github.com/golang/protobuf/protoc-gen-go@${arg_protoc_gen_go_ver}
# bind start_ide.sh
RUN           ln --symbolic /usr/bin/start_ide.sh    /usr/bin/idea \
&&            ln --symbolic /usr/bin/start_ide.sh    /usr/bin/goland
# install kubectl
RUN           curl -LO https://dl.k8s.io/release/v${arg_kubectl_ver}/bin/linux/amd64/kubectl \
&&            install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl \
&&            rm kubectl
# install helm3
RUN           curl -LO https://get.helm.sh/helm-v${arg_helm_ver}-linux-amd64.tar.gz \
&&            tar -xzf helm-v${arg_helm_ver}-linux-amd64.tar.gz \
&&            chmod +x linux-amd64/helm \
&&            mv linux-amd64/helm /usr/local/bin/helm \
&&            rm helm-v${arg_helm_ver}-linux-amd64.tar.gz \
&&            rm -rf linux-amd64

# copy kind from builder
COPY          --from=builder /kind/kind /usr/local/bin
RUN           chmod +x /usr/local/bin/kind

# set access rules
RUN           chmod -R a+rwx $GOPATH \
&&            chmod -R a+rwx $PROTOPATH


# add scripts required to properly setup running container
ADD           devkit \
              start_ide.sh \
              devkit-entrypoint.sh    /usr/bin/

# set entrypoint and default arguments
ENTRYPOINT    [ "devkit-entrypoint.sh" ]
