#!/bin/bash

#source ./others/install/ui/restartCheck.sh

ins_ui_restartCheck () {
    source ./others/common/log.sh
    
    local results=$?
    local returnValue=

    if [[ $results == 1 || $results == 255 ]] ; then
        com_log "ERROR" "process canceld, retrying... "
        returnValue=1
    else
        returnValue=0
    fi
    return $returnValue
}