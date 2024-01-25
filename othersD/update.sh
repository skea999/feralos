#!/bin/sh

common_log () {
    local typeV=$1
    local textV=$2
    local noColorV='\033[0m'
    local redColorV='\033[0;31m'
    local yellowColorV='\033[1;33m'
    local blueColorV='\033[1;34m'
    local colorV=$noColorV
    

    case "$typeV" in
        "INFO")
            colorV=$blueColorV
        ;;
        "WARN")
            colorV=$yellowColorV
        ;;
        "ERROR")
            colorV=$redColorV
        ;;
    esac

    echo
    echo -e "${colorV}$textV${noColorV}"
    echo
}

common_retryIfCommandFails () {
    #depends on common_log; 

    local commandToRunV=$1
    local -i returnValueV
    local -i numberRetryV=3
    local -i iV=1


    while [[ "$returnValueV" !=  "0" ]] && [[ $iV -le $numberRetryV ]] ; do
        echo
        
        ${commandToRunV}
        returnValueV=$?
        echo
        if [ $returnValueV != 0 ]; then
            common_log "ERROR" "$commandToRunV"
            common_log "WARN" "Retrying $iV time... "
            read -t 5 -p "Press enter to retry. I am going to wait for 5 seconds only ... "
        fi
        if [ $iV = $numberRetryV ]; then
            dialog --colors --default-button "yes" --yesno "\nDo you want to retry \n\Z1$commandToRunV \n that gived an error $iV times? " 0 0
            if [ $? = 1 ]; then
                common_log "ERROR" "Command $commandToRunV gived an error, pls fix it. "
                echo
            else
                iV=0
            fi
        fi
        ((iV=iV+1))
    done
}

common_retryIfCommandFails "sudo xbps-install -Syu xbps"
common_retryIfCommandFails "sudo xbps-install -Syu "

#common_retryIfCommandFails "omz update  "

#common_retryIfCommandFails "sudo snap refresh "

common_retryIfCommandFails "flatpak update --assumeyes "

common_retryIfCommandFails "lxc exec games apt-get update "
common_retryIfCommandFails "lxc exec games -- apt-get upgrade -y "
common_retryIfCommandFails "lxc exec games -- apt-get dist-upgrade -y "
common_retryIfCommandFails "lxc exec games -- apt-get dist-upgrade -y "
common_retryIfCommandFails "lxc exec games -- apt-get autoremove -y "
common_retryIfCommandFails "lxc exec games -- apt-get autoclean -y "

