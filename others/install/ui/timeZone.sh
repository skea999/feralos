#!/bin/bash

#########todo##########

#source ./others/install/ui/timeZone.sh

ins_ui_timeZone () {
    source ./others/install/ui/restartCheck.sh

    #timezonesList
    #ls -d /usr/share/zoneinfo/[[:upper:]]*/ | sed "s/\/usr\/share\/zoneinfo\///g" #i don't know if it usefull

    local resultTimeZoneFirstLevel
    resultTimeZoneFirstLevel=$(cat $com_chosenTimeZoneFileV | sed 's/\// /g' | awk '{print $4}')

    local resultTimeZoneSecondLevel
    resultTimeZoneSecondLevel=$(cat $com_chosenTimeZoneFileV | sed 's/\// /g' | awk '{print $5}')

    echo "" > $com_chosenTimeZoneFileV

    local timeZoneSecondLevel
    

    local args
    variableToArray () {
        local i
        local variable=$1
        args=()

        for i in $variable ; do
            args+=("$i" "")
        done
    }
    while true ; do

        local timeZoneFirstLevel=$(ls /usr/share/zoneinfo/ | grep "^[A-Z]")
        variableToArray "$timeZoneFirstLevel"
        resultTimeZoneFirstLevel=$(dialog --colors --title "\Z1\ZbTimeZone " --default-item "$resultTimeZoneFirstLevel" --menu "Select a TimeZone " 0 0 0 "${args[@]}" --output-fd 1)
        ins_ui_restartCheck || return 1

        local timeZoneSecondLevel=$(ls /usr/share/zoneinfo/$resultTimeZoneFirstLevel | sed "s/\/usr\/share\/zoneinfo\///g")
        variableToArray "$timeZoneSecondLevel"
        resultTimeZoneSecondLevel=$(dialog --colors --title "\Z1\ZbTimeZone " --default-item "$resultTimeZoneSecondLevel" --menu "Select a TimeZone " 0 0 0 "${args[@]}" --output-fd 1)
        ins_ui_restartCheck || continue
        break 2

    done

    if [ "$resultTimeZoneFirstLevel" = "$resultTimeZoneSecondLevel" ]; 
    then
        echo "/usr/share/zoneinfo/$resultTimeZoneFirstLevelFileV" > $com_chosenTimeZoneFileV
    else
        echo "/usr/share/zoneinfo/$resultTimeZoneFirstLevel/$resultTimeZoneSecondLevel" > $com_chosenTimeZoneFileV
    fi
    com_log "INFO" "TimeZone "
    com_log "WARN" "$(cat $com_chosenTimeZoneFileV)"
    return 0
}