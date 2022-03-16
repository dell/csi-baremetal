ARG KIND_BUILDER
FROM $KIND_BUILDER as builder

COPY kind/ /kind/

RUN chmod +x /kind/kind-build.sh \
&&  /kind/kind-build.sh /kind

FROM opensuse/leap:latest

ARG           arg_docker_ver
ARG           arg_go_ver
ARG           arg_golandci_ver
ARG           arg_kubectl_ver
ARG           arg_helm_ver

ENV           GOPATH="/usr/share/go"
ENV           GOROOT="/usr/local/go"
ENV           GOCACHE="$GOPATH/.cache/go-build"
ENV           GOENV="$GOPATH/.cache/go/env"
ENV           PATH="$PATH:$GOPATH/bin:$GOROOT/bin"


RUN           zypper --no-gpg-checks --non-interactive refresh \
&&            zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends \
              curl \
              docker-${arg_docker_ver} \
              gcc \
              git \
              libXi6 \
              libXtst6 \
              make \
              sudo \
              vim \
              wget \
              xorg-x11 \
              xorg-x11-fonts \
# install go
&&            wget https://go.dev/dl/go${arg_go_ver}.linux-amd64.tar.gz \
&&            tar -C /usr/local -xzf go${arg_go_ver}.linux-amd64.tar.gz \
&&            rm go${arg_go_ver}.linux-amd64.tar.gz \
# install bin golangci
&&            curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v${arg_golandci_ver} \
&&            chmod -R a+rwx $GOPATH \
# bind start_ide.sh
&&            ln --symbolic /usr/bin/start_ide.sh    /usr/bin/idea \
&&            ln --symbolic /usr/bin/start_ide.sh    /usr/bin/goland \
# install kubectl
&& curl -LO https://dl.k8s.io/release/v${arg_kubectl_ver}/bin/linux/amd64/kubectl \
&& install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl \
&& rm kubectl \
# install helm3
&& curl -LO https://get.helm.sh/helm-v${arg_helm_ver}-linux-amd64.tar.gz \
&& tar -xzf helm-v${arg_helm_ver}-linux-amd64.tar.gz \
&& chmod +x linux-amd64/helm \
&& mv linux-amd64/helm /usr/local/bin/helm \
&& rm helm-v${arg_helm_ver}-linux-amd64.tar.gz \
&& rm -rf linux-amd64

COPY --from=builder /kind/kind /usr/local/bin
RUN chmod +x /usr/local/bin/kind

# add scripts required to properly setup running container
ADD           devkit \
              start_ide.sh \
              devkit-entrypoint.sh    /usr/bin/

# set entrypoint and default arguments
ENTRYPOINT    [ "devkit-entrypoint.sh" ]
