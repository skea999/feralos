#!/bin/sh

functionsStartUi_init () {
    #depends on functionsCommon_log functionsCommon_retryIfCommandFails 

    rm -rf ~/.dialogrc
    functionsCommon_retryIfCommandFails "dialog --create-rc ~/.dialogrc"

    sed -i "s/OFF/ON/g" ~/.dialogrc
    functionsCommon_log "INFO" "sed -i \"s/OFF/ON/g\" ~/.dialogrc"
    #sed -i "s/CYAN/BLACK/g" ~/.dialogrc
    #functionsCommon_log "INFO" "sed -i \"s/CYAN/BLACK/g\" ~/.dialogrc"
    #sed -i "s/BLUE/CYAN/g" ~/.dialogrc
    #functionsCommon_log "INFO" "sed -i \"s/BLUE/CYAN/g\" ~/.dialogrc"
    #sed -i "s/GREEN/CYAN/g" ~/.dialogrc
    #functionsCommon_log "INFO" "sed -i \"s/GREEN/CYAN/g\" ~/.dialogrc"
    #sed -i "s/YELLOW/BLACK/g" ~/.dialogrc
    #functionsCommon_log "INFO" "sed -i \"s/YELLOW/BLACK/g\" ~/.dialogrc"
}