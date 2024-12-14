#!/bin/bash

#source ./others/install/ui/messageBox.sh

ins_ui_messageBox () {
    source ./others/common/log.sh

    local title=$1
    local bodyText=$2


    com_log "INFO" "$title"
    dialog --colors --title "\Z1\Zb$title " --msgbox "$bodyText " 0 0
}