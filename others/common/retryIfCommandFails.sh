#!/bin/bash

#source ./others/common/retryIfCommandFails.sh
#old verify_f
com_retryIfCommandFails () {
    source ./others/common/log.sh
    source ./others/common/stringToFile.sh

    source ./others/config/env.sh

    local commandToRun=$1
    local -i returnValue
    local -i numberRetry=3
    local -i i=1


    com_log "WARN" "Executing $commandToRun ... "
    while [[ "$returnValue" !=  "0" ]] && [[ $i -le $numberRetry ]] ; do
        echo
        eval "$commandToRun"
        returnValue=$?
        echo
        if [ $returnValue != 0 ]; then
            com_log "ERROR" "$commandToRun"
            com_log "WARN" "Retrying $i time... "
            read -t 5 -p "Press enter to retry. I am going to wait for 5 seconds only ... "
        fi
        if [ $i = $numberRetry ]; then
            dialog --colors --default-button "yes" --yesno "\nDo you want to retry \n\Z1$commandToRun \n that gied an error $i times? " 0 0
            if [ $? = 1 ]; then
                com_stringToFile "Command $commandToRun gied an error, pls fix it. " $con_logFile
                echo
            else
                i=0
            fi
        fi
        ((i=i+1))
        #echo $i
    done
}