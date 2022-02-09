FROM opensuse/leap:latest

ARG           arg_docker_ver
ARG           arg_go_ver
ARG           arg_golandci_ver

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
&&            ln --symbolic /usr/bin/start_ide.sh    /usr/bin/goland

# add scripts required to properly setup running container
ADD           devkit \
              start_ide.sh \
              devkit-entrypoint.sh    /usr/bin/

# set entrypoint and default arguments
ENTRYPOINT    [ "devkit-entrypoint.sh" ]
