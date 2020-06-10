#!/bin/bash

case $DRIVEMGR_MANAGER in
  "LOOPBACK")
    BIN_NAME=loopback-drivemgr
    ;;
  "IDRAC")
    BIN_NAME=idrac-drivemgr
    ;;
  "HAL")  # TODO: ADD JIRA NUBMER HERE
    BIN_NAME=hal-drivemgr
    ;;
esac

if [ -z ${BIN_NAME} ];then
  echo "ERROR: drive manager wasn't recognized"
  exit 1
fi

echo "Use ${BIN_NAME} file"

exec ./${BIN_NAME} "$@"
