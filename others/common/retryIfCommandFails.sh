#!/bin/sh

#old verify_f
functionsCommon_retryIfCommandFails () {
    #depends on functionsCommon_log; functionsCommon_stringToFile

    local functionsCommon_retryIfCommandFails_commandToRunV=$1
    local -i functionsCommon_retryIfCommandFails_returnValueV
    local -i functionsCommon_retryIfCommandFails_numberRetryV=3
    local -i functionsCommon_retryIfCommandFails_iV=1


    functionsCommon_log "WARN" "Executing $functionsCommon_retryIfCommandFails_commandToRunV ... " $functionsCommon_configurationLogFilePathV
    while [[ "$functionsCommon_retryIfCommandFails_returnValueV" !=  "0" ]] && [[ $functionsCommon_retryIfCommandFails_iV -le $functionsCommon_retryIfCommandFails_numberRetryV ]] ; do
        echo
        eval "$functionsCommon_retryIfCommandFails_commandToRunV"
        functionsCommon_retryIfCommandFails_returnValueV=$?
        echo
        if [ $functionsCommon_retryIfCommandFails_returnValueV != 0 ]; then
            functionsCommon_log "ERROR" "$functionsCommon_retryIfCommandFails_commandToRunV"
            functionsCommon_log "WARN" "Retrying $functionsCommon_retryIfCommandFails_iV time... "
            read -t 5 -p "Press enter to retry. I am going to wait for 5 seconds only ... "
        fi
        if [ $functionsCommon_retryIfCommandFails_iV = $functionsCommon_retryIfCommandFails_numberRetryV ]; then
            dialog --colors --default-button "yes" --yesno "\nDo you want to retry \n\Z1$functionsCommon_retryIfCommandFails_commandToRunV \n that gived an error $functionsCommon_retryIfCommandFails_iV times? " 0 0
            if [ $? = 1 ]; then
                functionsCommon_stringToFile "Command $functionsCommon_retryIfCommandFails_commandToRunV gived an error, pls fix it. " $functionsCommon_logFailedCommandsFileV
                echo
            else
                functionsCommon_retryIfCommandFails_iV=0
            fi
        fi
        ((functionsCommon_retryIfCommandFails_iV=functionsCommon_retryIfCommandFails_iV+1))
        #echo $iV
    done
}