#!/bin/bash

#source ./others/install/ui/yesNo.sh

ins_ui_yesNo () {
    source ./others/common/log.sh

    results=
    local defaultValue=$(cat $resultFile)
    case $defaultValue in
        0)
            defaultValue="yes"
        ;;
        1)
            defaultValue="no"
        ;;
        *)
            defaultValue="no"
        ;;
    esac

    #touch $resultFile
    echo "" > $resultFile

    local title=$1
    local bodyText=$2

    local returnValue=
    local app=""


    dialog --colors --title "\Z1\Zb$title " --default-button $defaultValue --yesno "\Z1$app\nDo you need $bodyText? " 0 0
    returnValue=$?
    results=$returnValue

    app="Please try again "
    com_log "INFO" "$title"
    com_log "INFO" "0 means yes 1 means no 255 means interrupted "
    com_log "WARN" "$results"
    return $returnValue

}