#!/bin/bash

#source ./others/install/ui/passwordBox.sh

ins_ui_passwordBox () {
    source ./others/common/log.sh
    source ./others/install/ui/restartCheck.sh

    local first=
    local second=
    results=
    local defaultValue=$(cat $resultFile)
    #touch $resultFile
    echo "" > $resultFile

    local title=$1
    local bodyText=$2

    local app=""
    

    while [ "$first" =  "" -o "$first" != "$second" ]; do
        first=$(dialog --colors --title "\Z1\Zb$title " --insecure --passwordbox "\Z1$app\nEnter $bodyText password: " 0 0 $defaultValue --output-fd 1)
        ins_ui_restartCheck || return 1

        second=$(dialog --colors --title "\Z1\Zb$title " --insecure --passwordbox "\Z1$app\nEnter $bodyText password for the second time: " 0 0 $defaultValue --output-fd 1)
        ins_ui_restartCheck || continue 

        app="Please try again"
    done
    com_log "INFO" "Password setup succesfully "
    results=$(echo $first)
    return 0
}