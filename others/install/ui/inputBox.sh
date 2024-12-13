#!/bin/sh

functionsStartUi_inputBox () {
    #depends on functionsCommon_log; 

    local functionsStartUi_inputBox_resultFileV=$1
    local functionsStartUi_inputBox_defaultValueV=$(cat $functionsStartUi_inputBox_resultFileV)
    #touch $functionsStartUi_inputBox_resultFileV
    echo "" > $functionsStartUi_inputBox_resultFileV

    local functionsStartUi_inputBox_returnValueV=
    local functionsStartUi_inputBox_titleV=$2
    local functionsStartUi_inputBox_bodyTextV=$3

    local functionsStartUi_inputBox_appV=""


    dialog --colors --title "\Z1\Zb$functionsStartUi_inputBox_titleV " --inputbox "\Z1$functionsStartUi_inputBox_appV\n$functionsStartUi_inputBox_bodyTextV" 0 0 $functionsStartUi_inputBox_defaultValueV 2> $functionsStartUi_inputBox_resultFileV
    functionsStartUi_inputBox_returnValueV=$?

    functionsStartUi_inputBox_appV="Please try again"

    functionsCommon_log "INFO" "$functionsStartUi_inputBox_titleV"
    functionsCommon_log "WARN" "$(cat $functionsStartUi_inputBox_resultFileV)"
    return $functionsStartUi_inputBox_returnValueV
}