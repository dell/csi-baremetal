#!/bin/bash

#
# This script starts IDEs made by JetBrains, such as IntelliJ IDEA and GoLand.
# Their IDEs usually put start scripts to <IDE_DIR>/bin/<ide>.sh.
# This script should not be used directly; symlink with the corresponding 
# IDE name should be created.
#

IDE="$(basename $0)"

START_SCRIPT=/opt/software/$IDE/bin/$IDE.sh

if [[ ! -x $START_SCRIPT ]]; then
    echo "ERROR: $START_SCRIPT does not exist or does not have executable permission." 1>&2
    exit 1
fi

$START_SCRIPT &>/dev/null &

# sleep for a while to give a process a chance to start
# if something is wrong, process will start and exit but we want it to start and work
sleep 2

# the following command will return error exit status if IDE not started
pidof -x $IDE.sh &>/dev/null

if [[ $? -ne 0 ]]; then
    echo "ERROR: $START_SCRIPT did not start; check that X11 support is enabled." 1>&2
    exit 2
fi

exit 0
