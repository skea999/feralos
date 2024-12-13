#!/bin/sh

#it works now, need to be integrated
functionsCommon_backupRestoreIFS () {
    local functionsCommon_backupRestoreIfs_savedIfsV=$IFS
    local functionsCommon_backupRestoreIFS_commandToRunV=$1
    IFS=$'\n'
    eval "$functionsCommon_backupRestoreIFS_commandToRunV"
    IFS=$functionsCommon_backupRestoreIfs_savedIfsV
}