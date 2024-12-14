#!/bin/bash

#source ./others/install/ui/init.sh

ins_ui_init () {
    source ./others/common/log.sh
    source ./others/common/retryIfCommandFails.sh

    rm -rf ~/.dialogrc
    com_retryIfCommandFails "dialog --create-rc ~/.dialogrc"

    sed -i "s/OFF/ON/g" ~/.dialogrc
    com_log "INFO" "sed -i \"s/OFF/ON/g\" ~/.dialogrc"
    #sed -i "s/CYAN/BLACK/g" ~/.dialogrc
    #com_log "INFO" "sed -i \"s/CYAN/BLACK/g\" ~/.dialogrc"
    #sed -i "s/BLUE/CYAN/g" ~/.dialogrc
    #com_log "INFO" "sed -i \"s/BLUE/CYAN/g\" ~/.dialogrc"
    #sed -i "s/GREEN/CYAN/g" ~/.dialogrc
    #com_log "INFO" "sed -i \"s/GREEN/CYAN/g\" ~/.dialogrc"
    #sed -i "s/YELLOW/BLACK/g" ~/.dialogrc
    #com_log "INFO" "sed -i \"s/YELLOW/BLACK/g\" ~/.dialogrc"
}