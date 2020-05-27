#!/bin/sh

echo enable_iscsi=true > /opt/emc/hal/etc/.hal_override

./drive-manager --drivemgrendpoint=tcp://localhost:8888
