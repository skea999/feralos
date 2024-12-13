#!/bin/sh

#it works now, need to be integrated
functionsCommon_backupRestoreIFS () {
    local savedIfs=$IFS
    local commandToRun=$1
    IFS=$'\n'
    eval "$commandToRun"
    IFS=$savedIfs
}