#!/bin/sh

functionsStartUi_restartCheck () {
    #depends on functionsCommon_log; 
    local functionsStartUi_restartCheck_resultsV=$?
    local functionsStartUi_restartCheck_returnValueV=

    if [[ $functionsStartUi_restartCheck_resultsV == 1 || $functionsStartUi_restartCheck_resultsV == 255 ]] ; then
        functionsCommon_log "ERROR" "process canceld, retrying... "
        functionsStartUi_restartCheck_returnValueV=1
    else
        functionsStartUi_restartCheck_returnValueV=0
    fi
    return $functionsStartUi_restartCheck_returnValueV
}