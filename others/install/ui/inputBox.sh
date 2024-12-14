#!/bin/bash

#source ./others/install/ui/inputBox.sh

ins_ui_inputBox () {
    source ./others/common/log.sh

    results=
    local defaultValue=$(cat $resultFile)
    #touch $resultFile
    echo "" > $resultFile

    local returnValue=
    local title=$1
    local bodyText=$2

    local app=""


    results=$(dialog --colors --title "\Z1\Zb$title " --inputbox "\Z1$app\n$bodyText" 0 0 $defaultValue)
    returnValue=$?

    app="Please try again"

    com_log "INFO" "$title"
    com_log "WARN" "$results"
    return $returnValue
}