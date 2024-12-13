#!/bin/sh

functionsStartUi_menuSelect () {
    #depends on functionsCommon_log; 

    local resultFile=$1
    local defaultValue=$(cat $resultFile)
    #touch $resultFile
    echo "" > $resultFile

    local title=$2
    local bodyText=$3
    local menuArgs=${4}

    local app=""
    local returnValue=
    local args=()
    local i

    for i in $menuArgs ; do
        args+=("$i" "")
    done


    dialog --colors --title "\Z1\Zb$title " --default-item "$defaultValue" --menu "\Z1$app\nSelect a $bodyText " 0 0 0 "${args[@]}" 2> $resultFile
    returnValue=$?

    app="Please try again "
    functionsCommon_log "INFO" "$title"
    functionsCommon_log "WARN" "$(cat $resultFile)"
    return $returnValue
}