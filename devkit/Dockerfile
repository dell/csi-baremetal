FROM opensuse/leap:latest

ARG           arg_docker_ver
ARG           arg_go_ver
ARG           arg_golandci_ver

ENV           GOPATH="/usr/share/go"
ENV           GOROOT="/usr/local/go"
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
# install golangci
&&            go install github.com/golangci/golangci-lint/cmd/golangci-lint@v${arg_golandci_ver} \
# bind start_ide.sh
&&            ln --symbolic /usr/bin/start_ide.sh    /usr/bin/idea \
&&            ln --symbolic /usr/bin/start_ide.sh    /usr/bin/goland

# set entrypoint and default arguments
ADD           devkit \
              start_ide.sh \
              devkit-entrypoint.sh    /usr/bin/

ENTRYPOINT    [ "devkit-entrypoint.sh" ]
