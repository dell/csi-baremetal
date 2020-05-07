FROM    asdrepo.isus.emc.com:8085/ecs/minimal:3.5.0.0-975.6b7cbbc.9

ENV     LC_ALL="en_US.utf8" \
        LANG="en_US.utf8" \
        GOPATH="/usr/share/go" \
        PATH="$PATH:/opt/emc/bin:/usr/local/go/bin:$GOPATH/bin" \
        LD_LIBRARY_PATH="$LD_LIBRARY_PATH:/opt/emc/hal/lib64" \
        HAL_VERSION="3.4.0.0-1835.b1a54fa"

RUN     zypper --no-gpg-checks --non-interactive addrepo http://asdrepo.isus.emc.com:8081/artifactory/ecsBase-local/any/dev ecsBase-any-dev \
&&      zypper --no-gpg-checks --non-interactive addrepo http://asdrepo.isus.emc.com:8081/artifactory/ecsBase-local/release-ecs-3.4/common base-release-ecs-3.4-common \
&&      zypper --no-gpg-checks --non-interactive refresh \
&&      zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends curl ipmitool glibc-locale wget gcc \
&&      ln --symbolic /usr/bin/xsdcxx /usr/bin/xsd \
&&      zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends http://asdrepo.isus.emc.com:8081/artifactory/ecs-prerelease-local/com/emc/asd/vipr/sles12/viprhal/viprhal/viprhal-${HAL_VERSION}-go.SLES/viprhal-${HAL_VERSION}.SLES.x86_64.rpm \
&&      zypper --no-gpg-checks --non-interactive install --auto-agree-with-licenses --no-recommends http://asdrepo.isus.emc.com:8081/artifactory/ecs-prerelease-local/com/emc/asd/vipr/sles12/viprhal/viprhal-devel/viprhal-${HAL_VERSION}-go.SLES/viprhal-devel-${HAL_VERSION}.SLES.x86_64.rpm

ADD     iscsi_entrypoint.sh /iscsi_entrypoint.sh
RUN     chmod 777 /iscsi_entrypoint.sh
# Set active template as halmgr/active_template/active_template.xml
COPY    halmgr/active_template/active_template.xml /opt/emc/hal/etc/active_template.xml

ADD     hwmgr hw-manager

EXPOSE  8888

ENTRYPOINT  ["./hw-manager"]
