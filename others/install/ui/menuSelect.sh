#!/bin/bash

#source ./others/install/ui/menuSelect.sh

ins_ui_menuSelect () {
    source ./others/common/log.sh

    results=
    local defaultValue=$(cat $resultFile)
    #touch $resultFile
    echo "" > $resultFile

    local title=$1
    local bodyText=$2
    local menuArgs=${3}

    local app=""
    local returnValue=
    local args=()
    local i

    for i in $menuArgs ; do
        args+=("$i" "")
    done


    results=$(dialog --colors --title "\Z1\Zb$title " --default-item "$defaultValue" --menu "\Z1$app\nSelect a $bodyText " 0 0 0 "${args[@]}")
    returnValue=$?

    app="Please try again "
    com_log "INFO" "$title"
    com_log "WARN" "$results"
    return $returnValue
}