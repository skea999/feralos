#!/bin/sh

#print a colored message, based on an argument taked as imput ("INFO" "ERROR" "WARNING"), like: functionsCommon_log "INFO" "commandToExecute"
functionsCommon_log () {
    local functionsCommon_log_typeV=$1
    local functionsCommon_log_textV=$2
    local functionsCommon_log_noColorV='\033[0m'
    local functionsCommon_log_redColorV='\033[0;31m'
    local functionsCommon_log_yellowColorV='\033[1;33m'
    local functionsCommon_log_blueColorV='\033[1;34m'
    local functionsCommon_log_colorV=$functionsCommon_log_noColorV


    case "$functionsCommon_log_typeV" in
        "INFO")
            functionsCommon_log_colorV=$functionsCommon_log_blueColorV
        ;;
        "WARN")
            functionsCommon_log_colorV=$functionsCommon_log_yellowColorV
        ;;
        "ERROR")
            functionsCommon_log_colorV=$functionsCommon_log_redColorV
        ;;
    esac

    echo $functionsCommon_log_textV >> $functionsCommon_configurationLogFilePathV

    echo
    echo -e "${functionsCommon_log_colorV}$functionsCommon_log_textV${functionsCommon_log_noColorV}"
    echo
}