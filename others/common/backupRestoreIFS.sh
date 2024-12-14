#!/bin/bash

#source ./others/common/backupRestoreIFS.sh
#it works now, need to be integrated
com_backupRestoreIFS () {
    local savedIfs=$IFS
    local commandToRun=$1
    IFS=$'\n'
    eval "$commandToRun"
    IFS=$savedIfs
}