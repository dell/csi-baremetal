docker create \
--name csi-baremetal-devkit \
--network host \
--ipc=host \
-v "/root/.ssh:/root/.ssh" \
-v "/home/anton/workspace:/workspace"  \
-v /etc/resolv.conf:/etc/resolv.conf \
-v /tmp:/tmp \
-e "EUID=0" \
-e "EGID=0" \
-e "USER_NAME=root" \
-e "STDOUT=true" \
--entrypoint="/bin/sh" csi-baremetal-devkit:latest \
"-c" "while [ ! -f {stop_dk_file} ]; do echo hello; sleep 2; done"

dv_name=`docker ps -a --format "{{.Names}}" |grep csi-baremetal-devkit|head -1`

docker start $dv_name
docker exec -it $dv_name bash