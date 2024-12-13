#!/bin/sh

functionsStartUi_passwordBox () {
    #depends on functionsCommon_log
    local functionsStartUi_passwordBox_firstV=
    local functionsStartUi_passwordBox_secondV=
    local functionsStartUi_passwordBox_resultFileV=$1
    local functionsStartUi_passwordBox_defaultValueV=$(cat $functionsStartUi_passwordBox_resultFileV)
    #touch $functionsStartUi_passwordBox_resultFileV
    echo "" > $functionsStartUi_passwordBox_resultFileV

    local functionsStartUi_passwordBox_titleV=$2
    local functionsStartUi_passwordBox_bodyTextV=$3

    local functionsStartUi_passwordBox_appV=""
    

    while [ "$functionsStartUi_passwordBox_firstV" =  "" -o "$functionsStartUi_passwordBox_firstV" != "$functionsStartUi_passwordBox_secondV" ]; do
        functionsStartUi_passwordBox_firstV=$(dialog --colors --title "\Z1\Zb$functionsStartUi_passwordBox_titleV " --insecure --passwordbox "\Z1$functionsStartUi_passwordBox_appV\nEnter $functionsStartUi_passwordBox_bodyTextV password: " 0 0 $functionsStartUi_passwordBox_defaultValueV --output-fd 1)
        functionsStartUi_restartCheck || return 1

        functionsStartUi_passwordBox_secondV=$(dialog --colors --title "\Z1\Zb$functionsStartUi_passwordBox_titleV " --insecure --passwordbox "\Z1$functionsStartUi_passwordBox_appV\nEnter $functionsStartUi_passwordBox_bodyTextV password for the second time: " 0 0 $functionsStartUi_passwordBox_defaultValueV --output-fd 1)
        functionsStartUi_restartCheck || continue 

        functionsStartUi_passwordBox_appV="Please try again"
    done
    functionsCommon_log "INFO" "Password setup succesfully "
    echo $functionsStartUi_passwordBox_firstV > $functionsStartUi_passwordBox_resultFileV
    return 0
}