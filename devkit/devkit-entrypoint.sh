#!/bin/bash

# variables from the environment
readonly EUID=${EUID:?"EUID environment variable should be defined"}
readonly EGID=${EGID:?"EGID environment variable should be defined"}
readonly USER_NAME=${USER_NAME:?"USER_NAME environment variable should be defined"}
readonly STDOUT=${STDOUT:?"STDOUT environment variable should be defined"}

readonly DOCKER_BASE_CMD='/usr/bin/dockerd --insecure-registry 0.0.0.0/0'
readonly USER_HOME_DIR="$([[ $EUID -eq 0 ]] && echo '/root' || echo "/home/$USER_NAME")"

# Jenkins docker plugin send cat command to check the created container and it should be ignored
CAT_OPT_VAL='cat'

readonly CMD_OPT='--cmd'
CMD_OPT_VAL=    # will be assigned later in the code if corresponding option was provided

readonly DOCKER_OPT="--docker"
# this is a default value; may be overriden if user explicitly provide corresponding option
DOCKER_OPT_VAL=yes

readonly IDEA_OPT='--idea'
# this is a default value; may be overriden if user explicitly provide corresponding option
IDEA_OPT_VAL=no

readonly GOLAND_OPT='--goland'
# this is a default value; may be overriden if user explicitly provide corresponding option
GOLAND_OPT_VAL=no

readonly DOCKER_STORAGE_DRIVER_OPT="--docker-storage-driver"
# this is a default value; may be overriden if user explicitly provide corresponding option
DOCKER_STORAGE_DRIVER_OPT_VAL=vfs

readonly HELP_OPT='--help'

readonly OPT_LST=( $HAL_OPT $CMD_OPT $DOCKER_OPT )

CMD_PROVIDED=false    # may be switched during analysis of input parameters
DOCKER_IN_DOCKER=true # may be switched during analysis of input parameters
START_IDEA=false      # may be switched during analysis of input parameters
START_GOLAND=false     # may be switched during analysis of input parameters

function usage() {
cat << EOF

Usage:
    devkit [option...]

Options:
    --docker arg  possible values: {
                                       yes, /* start docker daemon during startup */
                                       no,  /* do not start docker daemon during startup */
                                   }
                  default value: $DOCKER_OPT_VAL

    --docker-storage-driver arg    possible values: {
                                       <storage-driver>, /* Arbitrary, depends on the system setup. */
                                                         /* External and internal docker storage drivers might be incompatible. */
                                                         /* Examples: vfs, overlay2, devicemapper */
                                   }
                  default value: $DOCKER_STORAGE_DRIVER_OPT_VAL

    --idea arg    possible values: {
                                       yes, /* start idea during startup */
                                       no,  /* do not start idea during startup */
                                   }
                  default value: $IDEA_OPT_VAL

    --goland arg   possible values: {
                                       yes, /* start goland during startup */
                                       no,  /* do not start goland during startup */
                                   }
                  default value: $GOLAND_OPT_VAL

    --cmd arg     possible values: {
                                       "",                /* start bash session */
                                       "cmd1; cmd2; ...", /* execute provided bash commands */
                                   }
                  default value: ""

    --help        show this message and exit

Configuration:
    To tune devkit's behavior set the following environment variables on your host machine:
$(cat /usr/bin/devkit | grep ":-" | sed -e 's|=.\+$||' -e 's|readonly[[:space:]]\+|        |')

EOF
}

#
# Waits until Docker daemon starts responding
#
function wait_until_docker_is_up() {
    local retries=10
    while ! timeout 3 docker ps &>/dev/null; do
        sleep 1
        ((retries-=1)) || return 1
    done
    return 0
}

#
# function will hide stdout messages in non-interactive mode
#
function devkit_echo() {
    local fd=$1
    local msg="${2}"
    local msg_beg="$([[ -n "${3}" ]] && echo "${3}" || echo "true")"
    local msg_end="$([[ -n "${4}" ]] && echo "${4}" || echo "true")"

    local echo_flags=""
    if ! ${msg_end}; then
        echo_flags="-n"
    fi

    local prefix=""
    if ${msg_beg}; then
        if [[ $fd -eq 1 ]]; then
            prefix="DEVKIT: "
        else
            prefix="ERROR: "
        fi
    fi

    local final_fd
    if ! $STDOUT; then
        if [[ $fd -ne 1 ]]; then
            final_fd=$fd
        else
            final_fd="/dev/null"
        fi
    else
        final_fd=$fd
    fi

    local full_msg="${prefix}${msg}"

    echo ${echo_flags} "${full_msg}" 1>&${final_fd}
}

#
# function accepts conditional expression and outputs + or - based on the return value
#
function state() {
    local condition="$@"
    $condition && echo '+' || echo '-'
}

#
# creates user based on environment variable values
#
function create_user() {
    if [[ -z "$EUID" || -z "$EGID" || -z "$USER_NAME" ]]; then
        devkit_echo 2 "USER_NAME, EUID or EGID environment variables are not defined"
        return 1
    fi

    local ret=0


    if id --user $USER_NAME &>/dev/null; then
       # user already exists (container was started with 'docker start' instead of devkit wrapper)
       devkit_echo 1 \
           "'devkit' is started with 'docker start' command, which is generally not recommended. Please, set DEVKIT_DOCKER_RM_BOOL to false to use 'devkit' in a stateless mode."
       return 0
    fi

    /usr/sbin/groupadd --gid $EGID --non-unique devkit || ((ret++))

    local useradd_more_opts=
    if [[ ! -d "$USER_HOME_DIR" ]]; then
        useradd_more_opts=" --create-home "
    fi

    /usr/sbin/useradd --uid $EUID \
                      --gid $EGID \
                      --shell /bin/bash \
                      --base-dir /home \
                      --groups docker \
                      $useradd_more_opts \
                      $USER_NAME || ((ret++))
    echo "$USER_NAME ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/10-$USER_NAME-nopassword || ((ret++))

    # since /home/USER_NAME directory is auto created during container creation
    # it is needed to give user rw access to this directory
    chown $USER_NAME $USER_HOME_DIR || ((ret++))
    chmod u+rw $USER_HOME_DIR || ((ret++))

    return $ret
}

#
# setup $HOME/.bashrc file to provide usable environment
#
function setup_bashrc() {
    echo "alias exit='exit &>/dev/null'" >> $USER_HOME_DIR/.bashrc

    if [[ -n $DISPLAY ]]; then
        echo "export DISPLAY=$DISPLAY" >> $USER_HOME_DIR/.bashrc
    fi

    if [[ -n $XAUTHORITY ]]; then
        echo "export XAUTHORITY=$XAUTHORITY" >> $USER_HOME_DIR/.bashrc
    fi

    chmod a+x $USER_HOME_DIR/.bashrc

    return $?
}

#
# Starts JetBrains' IDE in case user provided corresponding option
#
function start_ide() {
    local ide="$1"
    local start_flag="START_${ide^^}"

    if ! ${!start_flag}; then
        return 0    # user do not want to start JetBrains' IDE
    fi

    echo "XAUTHORITY=$XAUTHORITY DISPLAY=$DISPLAY ${ide}" | su $USER_NAME

    return $?
}

#
# performs general business logic; all environment variables should be initialized
#
function start() {
    local rc=0
    local docker_pid

    devkit_echo 1 "docker daemon..." true false
    if $DOCKER_IN_DOCKER; then
        local docker_opts="--storage-driver=$DOCKER_STORAGE_DRIVER_OPT_VAL"
        $DOCKER_BASE_CMD $docker_opts &>/dev/null &
        docker_pid=$!
        if ! wait_until_docker_is_up; then
            devkit_echo 2 "failed to start docker daemon."
            return 2
        fi
    fi
    devkit_echo 1 ".............[$(state $DOCKER_IN_DOCKER)]" false true

    create_user || return 3

    setup_bashrc || return 4

    devkit_echo 1 "start IntelliJ IDEA..........[$(state $START_IDEA)]"

    start_ide idea || return 5

    devkit_echo 1 "start CLion..................[$(state $START_GOLAND)]"

    start_ide goland || return 6

    devkit_echo 1 "bash session.................[$(state true)]"

    if ! $CMD_PROVIDED; then
        su $USER_NAME <&0
    else
        su $USER_NAME -c "$CMD_OPT_VAL" <&0
    fi
    rc=$?

    if $DOCKER_IN_DOCKER; then
        kill -TERM $docker_pid
        wait $docker_pid
        wait $docker_pid
    fi

    return $rc
}

#
# Parse input options
#
while true; do
    case "$1" in
        $DOCKER_OPT)
            DOCKER_OPT_VAL="$2"
            if [[ "$DOCKER_OPT_VAL" == "no" ]]; then
                DOCKER_IN_DOCKER=false
            elif [[ -z "$DOCKER_OPT_VAL" || "$DOCKER_OPT_VAL" != "yes" ]]; then
                devkit_echo 2 "$DOCKER_OPT argument should be not empty and should be only yes/no"
                exit 2
            fi

            shift 2
            continue
        ;;

        $DOCKER_STORAGE_DRIVER_OPT)
            DOCKER_STORAGE_DRIVER_OPT_VAL="$2"
            if [[ -z "$DOCKER_OPT_VAL"  ]]; then
                devkit_echo 2 "$DOCKER_STORAGE_DRIVER_OPT argument should be not empty"
                exit 2
            fi

            shift 2
            continue
        ;;

        $CMD_OPT)
            CMD_OPT_VAL="$2"

            # remove leading whitespace characters
            CMD_OPT_VAL="${CMD_OPT_VAL#"${CMD_OPT_VAL%%[![:space:]]*}"}"
            # remove trailing whitespace characters
            CMD_OPT_VAL="${CMD_OPT_VAL%"${CMD_OPT_VAL##*[![:space:]]}"}"

            if [[ "${OPT_LST[@]}" =~ "$CMD_OPT_VAL" || -z "$CMD_OPT_VAL" ]]; then
                CMD_OPT_VAL=    # empty
                shift 1
            else
                CMD_PROVIDED=true
                shift 2
            fi
            continue
        ;;

        $IDEA_OPT)
            IDEA_OPT_VAL="$2"

            if [[ "$IDEA_OPT_VAL" == "yes" ]]; then
                START_IDEA=true
            elif [[ -z "$IDEA_OPT_VAL" || "$IDEA_OPT_VAL" != "no" ]]; then
                devkit_echo 2 "$IDEA_OPT argument should be not empty and should be only yes/no"
                exit 2
            fi

            shift 2
            continue
        ;;

        $GOLAND_OPT)
            GOLAND_OPT_VAL="$2"

            if [[ "$GOLAND_OPT_VAL" == "yes" ]]; then
                START_GOLAND=true
            elif [[ -z "$GOLAND_OPT_VAL" || "$GOLAND_OPT_VAL" != "no" ]]; then
                devkit_echo 2 "$GOLAND_OPT argument should be not empty and should be only yes/no"
                exit 2
            fi

            shift 2
            continue
        ;;

        $HELP_OPT)
            usage
            exit 0
        ;;

        $CAT_OPT_VAL)
            shift 2
            continue
        ;;

        *)
            [[ -z "$1" ]] && break
            devkit_echo 2 "unknown input parameter \`$1\`"
            exit 3
        ;;
    esac
done

start

exit $?
