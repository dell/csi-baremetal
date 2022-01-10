FROM opensuse/leap:latest

# TODO: args
#ARG           arg_docker_ver

ENV           GOPATH="/usr/share/go"
ENV           PATH="$PATH:/opt/emc/bin:/usr/local/go/bin:$GOPATH/bin"
# deprecated since 1.16 go version
ENV           GO111MODULE=on


RUN           zypper --no-gpg-checks --non-interactive refresh \
&&            zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends \
                  vim \
                  make \
                  curl \
                  wget \
                  git \
                  docker \
# install go
&&            wget https://go.dev/dl/go1.15.2.linux-amd64.tar.gz \
&&            tar -C /usr/local -xzf go1.15.2.linux-amd64.tar.gz \
&&            rm go1.15.2.linux-amd64.tar.gz \
# install golint
&&            wget https://github.com/golangci/golangci-lint/releases/download/v1.31.0/golangci-lint-1.31.0-linux-amd64.tar.gz \
&&            tar -C /usr/local -xzf golangci-lint-1.31.0-linux-amd64.tar.gz \
&&            rm golangci-lint-1.31.0-linux-amd64.tar.gz

# set entrypoint and default arguments
#ADD           entrypoint.sh    /usr/bin/
#ENTRYPOINT    [ "entrypoint.sh" ]