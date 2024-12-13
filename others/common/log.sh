#!/bin/sh

#print a colored message, based on an argument taked as imput ("INFO" "ERROR" "WARNING"), like: functionsCommon_log "INFO" "commandToExecute"
functionsCommon_log () {
    local type=$1
    local text=$2
    local noColor='\033[0m'
    local redColor='\033[0;31m'
    local yellowColor='\033[1;33m'
    local blueColor='\033[1;34m'
    local color=$noColor


    case "$type" in
        "INFO")
            color=$blueColor
        ;;
        "WARN")
            color=$yellowColor
        ;;
        "ERROR")
            color=$redColor
        ;;
    esac

    echo $text >> $functionsCommon_configurationLogFilePathV

    echo
    echo -e "${color}$text${noColor}"
    echo
}