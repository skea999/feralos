#!/bin/sh

functionsStartUi_yesNo () {
    #depends on functionsCommon_log; 

    local resultFile=$1
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

    local title=$2
    local bodyText=$3

    local returnValue=
    local app=""


    dialog --colors --title "\Z1\Zb$title " --default-button $defaultValue --yesno "\Z1$app\nDo you need $bodyText? " 0 0
    returnValue=$?
    echo $returnValue > $resultFile

    app="Please try again "
    functionsCommon_log "INFO" "$title"
    functionsCommon_log "INFO" "0 means yes 1 means no 255 means interrupted "
    functionsCommon_log "WARN" "$(cat $resultFile)"
    return $returnValue

}