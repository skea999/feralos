#!/bin/sh

functionsStartUi_messageBox () {
    #depends on functionsCommon_log

    local title=$1
    local bodyText=$2


    functionsCommon_log "INFO" "$title"
    dialog --colors --title "\Z1\Zb$title " --msgbox "$bodyText " 0 0
}