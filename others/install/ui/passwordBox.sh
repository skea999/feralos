#!/bin/sh

functionsStartUi_passwordBox () {
    #depends on functionsCommon_log
    local first=
    local second=
    local resultFile=$1
    local defaultValue=$(cat $resultFile)
    #touch $resultFile
    echo "" > $resultFile

    local title=$2
    local bodyText=$3

    local app=""
    

    while [ "$first" =  "" -o "$first" != "$second" ]; do
        first=$(dialog --colors --title "\Z1\Zb$title " --insecure --passwordbox "\Z1$app\nEnter $bodyText password: " 0 0 $defaultValue --output-fd 1)
        functionsStartUi_restartCheck || return 1

        second=$(dialog --colors --title "\Z1\Zb$title " --insecure --passwordbox "\Z1$app\nEnter $bodyText password for the second time: " 0 0 $defaultValue --output-fd 1)
        functionsStartUi_restartCheck || continue 

        app="Please try again"
    done
    functionsCommon_log "INFO" "Password setup succesfully "
    echo $first > $resultFile
    return 0
}