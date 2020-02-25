#!/bin/sh

echo enable_iscsi=true > /opt/emc/hal/etc/.hal_override

./hw-manager --hwmgrendpoint=tcp://localhost:8888
