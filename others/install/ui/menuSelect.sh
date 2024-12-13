#!/bin/sh

functionsStartUi_menuSelect () {
    #depends on functionsCommon_log; 

    local functionsStartUi_menuSelect_resultFileV=$1
    local functionsStartUi_menuSelect_defaultValueV=$(cat $functionsStartUi_menuSelect_resultFileV)
    #touch $functionsStartUi_menuSelect_resultFileV
    echo "" > $functionsStartUi_menuSelect_resultFileV

    local functionsStartUi_menuSelect_titleV=$2
    local functionsStartUi_menuSelect_bodyTextV=$3
    local functionsStartUi_menuSelect_menuArgsV=${4}

    local functionsStartUi_menuSelect_appV=""
    local functionsStartUi_menuSelect_returnValueV=
    local functionsStartUi_menuSelect_argsA=()
    local functionsStartUi_menuSelect_iV

    for functionsStartUi_menuSelect_iV in $functionsStartUi_menuSelect_menuArgsV ; do
        functionsStartUi_menuSelect_argsA+=("$functionsStartUi_menuSelect_iV" "")
    done


    dialog --colors --title "\Z1\Zb$functionsStartUi_menuSelect_titleV " --default-item "$functionsStartUi_menuSelect_defaultValueV" --menu "\Z1$functionsStartUi_menuSelect_appV\nSelect a $functionsStartUi_menuSelect_bodyTextV " 0 0 0 "${functionsStartUi_menuSelect_argsA[@]}" 2> $functionsStartUi_menuSelect_resultFileV
    functionsStartUi_menuSelect_returnValueV=$?

    appV="Please try again "
    functionsCommon_log "INFO" "$functionsStartUi_menuSelect_titleV"
    functionsCommon_log "WARN" "$(cat $functionsStartUi_menuSelect_resultFileV)"
    return $functionsStartUi_menuSelect_returnValueV
}