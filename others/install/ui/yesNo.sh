#!/bin/sh

functionsStartUi_yesNo () {
    #depends on functionsCommon_log; 

    local functionsStartUi_yesNo_resultFileV=$1
    local functionsStartUi_yesNo_defaultValueV=$(cat $functionsStartUi_yesNo_resultFileV)
    case $functionsStartUi_yesNo_defaultValueV in
        0)
            functionsStartUi_yesNo_defaultValueV="yes"
        ;;
        1)
            functionsStartUi_yesNo_defaultValueV="no"
        ;;
        *)
            functionsStartUi_yesNo_defaultValueV="no"
        ;;
    esac

    #touch $functionsStartUi_yesNo_resultFileV
    echo "" > $functionsStartUi_yesNo_resultFileV

    local functionsStartUi_yesNo_titleV=$2
    local functionsStartUi_yesNo_bodyTextV=$3

    local functionsStartUi_yesNo_returnValueV=
    local functionsStartUi_yesNo_appV=""


    dialog --colors --title "\Z1\Zb$functionsStartUi_yesNo_titleV " --default-button $functionsStartUi_yesNo_defaultValueV --yesno "\Z1$functionsStartUi_yesNo_appV\nDo you need $functionsStartUi_yesNo_bodyTextV? " 0 0
    functionsStartUi_yesNo_returnValueV=$?
    echo $functionsStartUi_yesNo_returnValueV > $functionsStartUi_yesNo_resultFileV

    appV="Please try again "
    functionsCommon_log "INFO" "$functionsStartUi_yesNo_titleV"
    functionsCommon_log "INFO" "0 means yes 1 means no 255 means interrupted "
    functionsCommon_log "WARN" "$(cat $functionsStartUi_yesNo_resultFileV)"
    return $functionsStartUi_yesNo_returnValueV

}