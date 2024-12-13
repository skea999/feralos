#!/bin/sh

functionsStartUi_inputBox () {
    #depends on functionsCommon_log; 

    local resultFile=$1
    local defaultValue=$(cat $resultFile)
    #touch $resultFile
    echo "" > $resultFile

    local returnValue=
    local title=$2
    local bodyText=$3

    local app=""


    dialog --colors --title "\Z1\Zb$title " --inputbox "\Z1$app\n$bodyText" 0 0 $defaultValue 2> $resultFile
    returnValue=$?

    app="Please try again"

    functionsCommon_log "INFO" "$title"
    functionsCommon_log "WARN" "$(cat $resultFile)"
    return $returnValue
}