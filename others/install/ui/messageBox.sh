#!/bin/sh

functionsStartUi_messageBox () {
    #depends on functionsCommon_log

    local functionsStartUi_messageBox_titleV=$1
    local functionsStartUi_messageBox_bodyTextV=$2


    functionsCommon_log "INFO" "$functionsStartUi_messageBox_titleV"
    dialog --colors --title "\Z1\Zb$functionsStartUi_messageBox_titleV " --msgbox "$functionsStartUi_messageBox_bodyTextV " 0 0
}