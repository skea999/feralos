#!/bin/sh

functionsStartUi_restartCheck () {
    #depends on functionsCommon_log; 
    local results=$?
    local returnValue=

    if [[ $results == 1 || $results == 255 ]] ; then
        functionsCommon_log "ERROR" "process canceld, retrying... "
        returnValue=1
    else
        returnValue=0
    fi
    return $returnValue
}