#!/bin/sh

#old verify_f
functionsCommon_retryIfCommandFails () {
    #depends on functionsCommon_log; functionsCommon_stringToFile

    local commandToRun=$1
    local -i returnValue
    local -i numberRetry=3
    local -i i=1


    functionsCommon_log "WARN" "Executing $commandToRun ... " $functionsCommon_configurationLogFilePathV
    while [[ "$returnValue" !=  "0" ]] && [[ $i -le $numberRetry ]] ; do
        echo
        eval "$commandToRun"
        returnValue=$?
        echo
        if [ $returnValue != 0 ]; then
            functionsCommon_log "ERROR" "$commandToRun"
            functionsCommon_log "WARN" "Retrying $i time... "
            read -t 5 -p "Press enter to retry. I am going to wait for 5 seconds only ... "
        fi
        if [ $i = $numberRetry ]; then
            dialog --colors --default-button "yes" --yesno "\nDo you want to retry \n\Z1$commandToRun \n that gied an error $i times? " 0 0
            if [ $? = 1 ]; then
                functionsCommon_stringToFile "Command $commandToRun gied an error, pls fix it. " $functionsCommon_logFailedCommandsFileV
                echo
            else
                i=0
            fi
        fi
        ((i=i+1))
        #echo $i
    done
}