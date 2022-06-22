#!/bin/bash

# clean shutdown
trap "stop; exit 0;" SIGTERM SIGINT

stop()
{
  echo "SIGTERM caught, terminating NFS process(es)..."
  /usr/sbin/exportfs -uav
  /usr/sbin/rpc.nfsd 0
  pid1=`pidof rpc.nfsd`
  pid2=`pidof rpc.mountd`
  # For IPv6 bug:
  pid3=`pidof rpcbind`
  kill -TERM $pid1 $pid2 $pid3 > /dev/null 2>&1
  echo "Terminated."
  exit
}

if [ -z "${MNT_DIR}" ]; then
  echo "MNT_DIR is missing -- it will be '/var'"
  MNT_DIR="/var"
fi
mkdir -p $MNT_DIR
if [ -z "${MNT_OPT}" ]; then
  echo "MNT_OPT is missing -- it will be async"
  MNT_OPT="async"
fi
echo "Writing MNT_DIR($MNT_DIR) to /etc/exports file"
echo "${MNT_DIR} *(rw,fsid=0,$MNT_OPT,no_subtree_check,no_auth_nlm,insecure,no_root_squash)" > /etc/exports

# http://redsymbol.net/articles/unofficial-bash-strict-mode/
set -uo pipefail
IFS=$'\n\t'

# runs till started up successfully
while true; do
  # is NFS running?
  pid=`pidof rpc.mountd`
  # $pid is null => start / restart NFS:
  while [ -z "$pid" ]; do
    echo "/etc/exports: $(cat /etc/exports)"
 
    /sbin/rpcbind -w
    /usr/sbin/rpc.nfsd --debug 8 --no-udp
    /usr/sbin/exportfs -rv
    /usr/sbin/rpc.mountd --debug all --no-udp

    sleep 1
    pid=`pidof rpc.mountd`
    if [ -z "$pid" ]; then
      echo "startup failure => retrying in 3"
      sleep 3
    fi
  done
  echo "ready"
  break
done

while true; do

  pid=`pidof rpc.mountd`
  if [ -z "$pid" ]; then
    echo "NSF failed => exiting"
    break
  fi
  sleep 3
done

sleep 1
exit 1
