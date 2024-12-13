#!/bin/sh

functionsStartUi_timeZone () {
    #timezonesList
    #ls -d /usr/share/zoneinfo/[[:upper:]]*/ | sed "s/\/usr\/share\/zoneinfo\///g" #i don't know if it usefull

    local functionsStartUi_timeZone_resultTimeZoneFirstLevelV
    functionsStartUi_timeZone_resultTimeZoneFirstLevelV=$(cat $functionsCommon_chosenTimeZoneFileV | sed 's/\// /g' | awk '{print $4}')

    local functionsStartUi_timeZone_resultTimeZoneSecondLevelV
    functionsStartUi_timeZone_resultTimeZoneSecondLevelV=$(cat $functionsCommon_chosenTimeZoneFileV | sed 's/\// /g' | awk '{print $5}')

    echo "" > $functionsCommon_chosenTimeZoneFileV

    local functionsStartUi_timeZone_timeZoneSecondLevelV
    

    local functionsStartUi_timeZone_argsA
    functionsStartUi_timeZone_variableToArray () {
        local functionsStartUi_timeZone_variableToArray_iV
        local functionsStartUi_timeZone_variableToArray_variableV=$1
        functionsStartUi_timeZone_argsA=()

        for functionsStartUi_timeZone_variableToArray_iV in $functionsStartUi_timeZone_variableToArray_variableV ; do
            functionsStartUi_timeZone_argsA+=("$functionsStartUi_timeZone_variableToArray_iV" "")
        done
    }
    while true ; do

        local functionsStartUi_timeZone_timeZoneFirstLevelV=$(ls /usr/share/zoneinfo/ | grep "^[A-Z]")
        functionsStartUi_timeZone_variableToArray "$functionsStartUi_timeZone_timeZoneFirstLevelV"
        functionsStartUi_timeZone_resultTimeZoneFirstLevelV=$(dialog --colors --title "\Z1\ZbTimeZone " --default-item "$functionsStartUi_timeZone_resultTimeZoneFirstLevelV" --menu "Select a TimeZone " 0 0 0 "${functionsStartUi_timeZone_argsA[@]}" --output-fd 1)
        functionsStartUi_restartCheck || return 1

        local functionsStartUi_timeZone_timeZoneSecondLevelV=$(ls /usr/share/zoneinfo/$functionsStartUi_timeZone_resultTimeZoneFirstLevelV | sed "s/\/usr\/share\/zoneinfo\///g")
        functionsStartUi_timeZone_variableToArray "$functionsStartUi_timeZone_timeZoneSecondLevelV"
        functionsStartUi_timeZone_resultTimeZoneSecondLevelV=$(dialog --colors --title "\Z1\ZbTimeZone " --default-item "$functionsStartUi_timeZone_resultTimeZoneSecondLevelV" --menu "Select a TimeZone " 0 0 0 "${functionsStartUi_timeZone_argsA[@]}" --output-fd 1)
        functionsStartUi_restartCheck || continue
        break 2

    done

    if [ "$functionsStartUi_timeZone_resultTimeZoneFirstLevelV" = "$functionsStartUi_timeZone_resultTimeZoneSecondLevelV" ]; 
    then
        echo "/usr/share/zoneinfo/$functionsStartUi_timeZone_resultTimeZoneFirstLevelFileV" > $functionsCommon_chosenTimeZoneFileV
    else
        echo "/usr/share/zoneinfo/$functionsStartUi_timeZone_resultTimeZoneFirstLevelV/$functionsStartUi_timeZone_resultTimeZoneSecondLevelV" > $functionsCommon_chosenTimeZoneFileV
    fi
    functionsCommon_log "INFO" "TimeZone "
    functionsCommon_log "WARN" "$(cat $functionsCommon_chosenTimeZoneFileV)"
    return 0
}