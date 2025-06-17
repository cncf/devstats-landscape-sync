#!/bin/bash
date
lock_file="/tmp/check_sync.lock"
if [ -f "${lock_file}" ]
then
  echo "$0: another check_sync instance is still running, exiting"
  exit 2
fi
if [ -z "$CHECK_SYNC_DIR" ]
then
  CHECK_SYNC_DIR="${HOME}/go/src/github.com/cncf/devstats-landscape-sync"
fi
cd "$CHECK_SYNC_DIR" || exit 5
git pull || exit 6
make || exit 7
function cleanup {
  rm -rf "${lock_file}"
}
> "${lock_file}"
trap cleanup EXIT
./check_sync 2>&1 | tee -a run.log
