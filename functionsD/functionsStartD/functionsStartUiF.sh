#!/bin/bash
source /scriptFilesD/functionsD/functionsCommonF.sh
source /scriptFilesD/othersD/othersDistributionSpecificThingsF.sh

functionsStartUi_benchmarkMirrorsV=
functionsStartUi_otherSoftwareFileV=/scriptFilesD/othersD/othersSoftwareToInstallF 
functionsStartUi_chosenDiskFileV=/scriptFilesD/chosenDiskF 
functionsStartUi_ultimateCheckFileV=/scriptFilesD/ultimateCheckF



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

#old messagebox_f
functionsStartUi_messageBox () {
    #depends on functionsCommon_log

    local functionsStartUi_messageBox_titleV=$1
    local functionsStartUi_messageBox_bodyTextV=$2


    functionsCommon_log "INFO" "$functionsStartUi_messageBox_titleV"
    dialog --colors --title "\Z1\Zb$functionsStartUi_messageBox_titleV " --msgbox "$functionsStartUi_messageBox_bodyTextV " 0 0
}

functionsStartUi_mirrorsBenchmark () {
    #depends on distributionSpecificThingF; functionsCommon_log; functionsStartUi_benchmarkMirrorsA
    functionsCommon_log "INFO" "Waiting for benchmark... "
    local functionsStartUi_mirrorsBenchmark_urlV=$othersDistributionSpecificThings_urlV
    local functionsStartUi_mirrorsBenchmark_regionsV=$(curl -s $functionsStartUi_mirrorsBenchmark_urlV | sed 's/<[^>]*>//g' | grep https:// | sed 's/\([A-Z]\)/ \1/g' | awk '{ s = ""; for (i = 2; i <= NF; i++) s = s $i " "; print s }')
    local functionsStartUi_mirrorsBenchmark_linksV=$(curl -s $functionsStartUi_mirrorsBenchmark_urlV | sed 's/<[^>]*>//g' | grep https:// | sed 's/\([A-Z]\)/ \1/g' | awk '{print $1}')
    
        #SPEED_F="speed.txt"
        #echo "" > $SPEED_F
    
    local functionsStartUi_mirrorsBenchmark_bestMirrorV
    local functionsStartUi_mirrorsBenchmark_TargetPackageV=$othersDistributionSpecificThings_TargetPackageV

    local functionsStartUi_mirrorsBenchmark_packageV
    
    local functionsStartUi_mirrorsBenchmark_pingV
    local functionsStartUi_mirrorsBenchmark_formattedSpeedV
    local functionsStartUi_mirrorsBenchmark_speedA


    functionsStartUi_mirrorsBenchmark_findPackage () {
        local functionsStartUi_mirrorsBenchmark_findPackage_listingUrlV=$othersDistributionSpecificThings_listingUrlV
        local functionsStartUi_mirrorsBenchmark_desiredPackageV=$1
        functionsStartUi_mirrorsBenchmark_packageV=$(curl -s $functionsStartUi_mirrorsBenchmark_findPackage_listingUrlV | sed 's/<[^>]*>//g' | grep $functionsStartUi_mirrorsBenchmark_desiredPackageV | grep "x86_64-musl.xbps " | awk '{print $1}' | head -n 1)
    }
    
    functionsStartUi_mirrorsBenchmark_speedTest () {
        local functionsStartUi_mirrorsBenchmark_speedTest_rootUrlV=$1
        local functionsStartUi_mirrorsBenchmark_subPathV=$othersDistributionSpecificThings_subPathV

        local functionsStartUi_mirrorsBenchmark_speedTest_urlV=${functionsStartUi_mirrorsBenchmark_speedTest_rootUrlV}${functionsStartUi_mirrorsBenchmark_subPathV}${functionsStartUi_mirrorsBenchmark_packageV}
        local functionsStartUi_mirrorsBenchmark_speedTest_hostV=$(echo $functionsStartUi_mirrorsBenchmark_speedTest_rootUrlV | awk -F/ '{print $3}')
        functionsStartUi_mirrorsBenchmark_pingV=`ping -c 1 ${functionsStartUi_mirrorsBenchmark_speedTest_hostV} | tail -1 | awk '{print $4}' | cut -d '/' -f 2 | bc -l`
        local functionsStartUi_mirrorsBenchmark_speedTest_speedV=$(curl -qfs -w '%{speed_download}' -o /dev/null --url "$functionsStartUi_mirrorsBenchmark_speedTest_urlV")
        functionsStartUi_mirrorsBenchmark_formattedSpeedV=$(numfmt --to=iec <<<"$functionsStartUi_mirrorsBenchmark_speedTest_speedV")
    }
    
    #we have a function for this
    local functionsStartUi_mirrorsBenchmark_saveIfsV=$IFS
    IFS=$'\n'
    functionsStartUi_mirrorsBenchmark_regionsV=($functionsStartUi_mirrorsBenchmark_regionsV)
    functionsStartUi_mirrorsBenchmark_linksV=($functionsStartUi_mirrorsBenchmark_linksV)
    IFS=$functionsStartUi_mirrorsBenchmark_saveIfsV

    functionsStartUi_mirrorsBenchmark_findPackage $functionsStartUi_mirrorsBenchmark_TargetPackageV

    local functionsStartUi_mirrorsBenchmark_oneRegionV
    local functionsStartUi_mirrorsBenchmark_iV
    for (( functionsStartUi_mirrorsBenchmark_iV=0; functionsStartUi_mirrorsBenchmark_iV<${#functionsStartUi_mirrorsBenchmark_linksV[@]}; functionsStartUi_mirrorsBenchmark_iV++ ))
    do
        functionsStartUi_mirrorsBenchmark_speedTest ${functionsStartUi_mirrorsBenchmark_linksV[$functionsStartUi_mirrorsBenchmark_iV]}
        functionsStartUi_mirrorsBenchmark_oneRegionV=$(echo ${functionsStartUi_mirrorsBenchmark_regionsV[$functionsStartUi_mirrorsBenchmark_iV]} | sed 's/ //g')
        
        functionsStartUi_mirrorsBenchmark_speedA+=($(echo -e "${functionsStartUi_mirrorsBenchmark_linksV[$functionsStartUi_mirrorsBenchmark_iV]}\t$functionsStartUi_mirrorsBenchmark_formattedSpeedV\t$functionsStartUi_mirrorsBenchmark_pingV\t${functionsStartUi_mirrorsBenchmark_oneRegionV}" | awk '{print $1 " " $2 "B/s-ping:" $3 "ms_" $4}' | head -n -1))
        #echo -e "${functionsStartUi_mirrorsBenchmark_linksV[$functionsStartUi_mirrorsBenchmark_iV]}\t$functionsStartUi_mirrorsBenchmark_formattedSpeedV\t$functionsStartUi_mirrorsBenchmark_pingV\t${functionsStartUi_mirrorsBenchmark_oneRegionV}" >> $SPEED_F
        functionsCommon_log "WARN" "Speed for ${functionsStartUi_mirrorsBenchmark_linksV[$functionsStartUi_mirrorsBenchmark_iV]} is: ${functionsStartUi_mirrorsBenchmark_formattedSpeedV}B/s ping: ${functionsStartUi_mirrorsBenchmark_pingV}ms Region: $functionsStartUi_mirrorsBenchmark_oneRegionV"
    done
    
    #sort file by speed
    #for functionsStartUi_mirrorsBenchmark_iV in ${functionsStartUi_mirrorsBenchmark_speedA[@]} ; do
    functionsStartUi_mirrorsBenchmark_saveIfsV=$IFS
    IFS=$'\n' 
    functionsStartUi_benchmarkMirrorsV=($(sort -r -n -k 2 <<<"${functionsStartUi_mirrorsBenchmark_speedA[*]}"))
    IFS=$functionsStartUi_mirrorsBenchmark_saveIfsV
        
    functionsCommon_log "WARN" "$functionsStartUi_benchmarkMirrorsV"
        #functionsStartUi_benchmarkMirrorsV+=$(echo -e "${functionsStartUi_mirrorsBenchmark_speedA}" | sort -r -n -k 2)
    #done
    #sort -r -n -k 2 -o $SPEED_F $SPEED_F 

    functionsStartUi_mirrorsBenchmark_bestMirrorV=$(cat $functionsStartUi_benchmarkMirrorsV | awk '{print $1}') # | head -n 1
    functionsCommon_log "WARN" "Best mirror is: $functionsStartUi_mirrorsBenchmark_bestMirrorV"
    #echo "Best mirror is: $functionsStartUi_mirrorsBenchmark_bestMirrorV"
    
    #cat $SPEED_F | awk '{print $1 " " $2 "B/s-ping:" $3 "ms_" $4}' | head -n -1 > ${SPEED_F}"_new"
    #cat ${SPEED_F}"_new" > $SPEED_F
    #rm ${SPEED_F}"_new"
}

#old inputbox_f
functionsStartUi_inputBox () {
    #depends on functionsCommon_log; 

    local functionsStartUi_inputBox_resultFileV=$1
    local functionsStartUi_inputBox_defaultValueV=$(cat $functionsStartUi_inputBox_resultFileV)
    #touch $functionsStartUi_inputBox_resultFileV
    echo "" > $functionsStartUi_inputBox_resultFileV

    local functionsStartUi_inputBox_returnValueV=
    local functionsStartUi_inputBox_titleV=$2
    local functionsStartUi_inputBox_bodyTextV=$3

    local functionsStartUi_inputBox_appV=""


    dialog --colors --title "\Z1\Zb$functionsStartUi_inputBox_titleV " --inputbox "\Z1$functionsStartUi_inputBox_appV\n$functionsStartUi_inputBox_bodyTextV" 0 0 $functionsStartUi_inputBox_defaultValueV 2> $functionsStartUi_inputBox_resultFileV
    functionsStartUi_inputBox_returnValueV=$?

    functionsStartUi_inputBox_appV="Please try again"

    functionsCommon_log "INFO" "$functionsStartUi_inputBox_titleV"
    functionsCommon_log "WARN" "$(cat $functionsStartUi_inputBox_resultFileV)"
    return $functionsStartUi_inputBox_returnValueV
}

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

#old passwordbox_f
functionsStartUi_passwordBox () {
    #depends on functionsCommon_log
    local functionsStartUi_passwordBox_firstV=
    local functionsStartUi_passwordBox_secondV=
    local functionsStartUi_passwordBox_resultFileV=$1
    local functionsStartUi_passwordBox_defaultValueV=$(cat $functionsStartUi_passwordBox_resultFileV)
    #touch $functionsStartUi_passwordBox_resultFileV
    echo "" > $functionsStartUi_passwordBox_resultFileV

    local functionsStartUi_passwordBox_titleV=$2
    local functionsStartUi_passwordBox_bodyTextV=$3

    local functionsStartUi_passwordBox_appV=""
    

    while [ "$functionsStartUi_passwordBox_firstV" =  "" -o "$functionsStartUi_passwordBox_firstV" != "$functionsStartUi_passwordBox_secondV" ]; do
        functionsStartUi_passwordBox_firstV=$(dialog --colors --title "\Z1\Zb$functionsStartUi_passwordBox_titleV " --insecure --passwordbox "\Z1$functionsStartUi_passwordBox_appV\nEnter $functionsStartUi_passwordBox_bodyTextV password: " 0 0 $functionsStartUi_passwordBox_defaultValueV --output-fd 1)
        functionsStartUi_restartCheck || return 1

        functionsStartUi_passwordBox_secondV=$(dialog --colors --title "\Z1\Zb$functionsStartUi_passwordBox_titleV " --insecure --passwordbox "\Z1$functionsStartUi_passwordBox_appV\nEnter $functionsStartUi_passwordBox_bodyTextV password for the second time: " 0 0 $functionsStartUi_passwordBox_defaultValueV --output-fd 1)
        functionsStartUi_restartCheck || continue 

        functionsStartUi_passwordBox_appV="Please try again"
    done
    functionsCommon_log "INFO" "Password setup succesfully "
    echo $functionsStartUi_passwordBox_firstV > $functionsStartUi_passwordBox_resultFileV
    return 0
}

#old menu_select_f
functionsStartUi_menuSelect () {
    #depends on functionsCommon_log; 

    local functionsStartUi_menuSelect_resultFileV=$1
    local functionsStartUi_menuSelect_defaultValueV=$(cat $functionsStartUi_menuSelect_resultFileV)
    #touch $functionsStartUi_menuSelect_resultFileV
    echo "" > $functionsStartUi_menuSelect_resultFileV

    local functionsStartUi_menuSelect_titleV=$2
    local functionsStartUi_menuSelect_bodyTextV=$3
    local functionsStartUi_menuSelect_menuArgsV=${4}

    local functionsStartUi_menuSelect_appV=""
    local functionsStartUi_menuSelect_returnValueV=
    local functionsStartUi_menuSelect_argsA=()
    local functionsStartUi_menuSelect_iV

    for functionsStartUi_menuSelect_iV in $functionsStartUi_menuSelect_menuArgsV ; do
        functionsStartUi_menuSelect_argsA+=("$functionsStartUi_menuSelect_iV" "")
    done


    dialog --colors --title "\Z1\Zb$functionsStartUi_menuSelect_titleV " --default-item "$functionsStartUi_menuSelect_defaultValueV" --menu "\Z1$functionsStartUi_menuSelect_appV\nSelect a $functionsStartUi_menuSelect_bodyTextV " 0 0 0 "${functionsStartUi_menuSelect_argsA[@]}" 2> $functionsStartUi_menuSelect_resultFileV
    functionsStartUi_menuSelect_returnValueV=$?

    appV="Please try again "
    functionsCommon_log "INFO" "$functionsStartUi_menuSelect_titleV"
    functionsCommon_log "WARN" "$(cat $functionsStartUi_menuSelect_resultFileV)"
    return $functionsStartUi_menuSelect_returnValueV
}
functionsStartUi_timeZone () {
    #timezonesList
    #ls -d /usr/share/zoneinfo/[[:upper:]]*/ | sed "s/\/usr\/share\/zoneinfo\///g" #i don't know if it usefull

    local functionsStartUi_timeZone_resultTimeZoneFirstLevelV
    functionsStartUi_timeZone_resultTimeZoneFirstLevelV=$(cat $functionsCommon_chosenTimeZoneFileV | sed 's/\// /g' | awk '{print $4}')

    local functionsStartUi_timeZone_resultTimeZoneSecondLevelV
    functionsStartUi_timeZone_resultTimeZoneSecondLevelV=$(cat $functionsCommon_chosenTimeZoneFileV | sed 's/\// /g' | awk '{print $5}')

    echo "" > $functionsCommon_chosenTimeZoneFileV

    local functionsStartUi_timeZone_timeZoneSecondLevelV
    

    local functionsStartUi_timeZone_argsA
    functionsStartUi_timeZone_variableToArray () {
        local functionsStartUi_timeZone_variableToArray_iV
        local functionsStartUi_timeZone_variableToArray_variableV=$1
        functionsStartUi_timeZone_argsA=()

        for functionsStartUi_timeZone_variableToArray_iV in $functionsStartUi_timeZone_variableToArray_variableV ; do
            functionsStartUi_timeZone_argsA+=("$functionsStartUi_timeZone_variableToArray_iV" "")
        done
    }
    while true ; do

        local functionsStartUi_timeZone_timeZoneFirstLevelV=$(ls /usr/share/zoneinfo/ | grep "^[A-Z]")
        functionsStartUi_timeZone_variableToArray "$functionsStartUi_timeZone_timeZoneFirstLevelV"
        functionsStartUi_timeZone_resultTimeZoneFirstLevelV=$(dialog --colors --title "\Z1\ZbTimeZone " --default-item "$functionsStartUi_timeZone_resultTimeZoneFirstLevelV" --menu "Select a TimeZone " 0 0 0 "${functionsStartUi_timeZone_argsA[@]}" --output-fd 1)
        functionsStartUi_restartCheck || return 1

        local functionsStartUi_timeZone_timeZoneSecondLevelV=$(ls /usr/share/zoneinfo/$functionsStartUi_timeZone_resultTimeZoneFirstLevelV | sed "s/\/usr\/share\/zoneinfo\///g")
        functionsStartUi_timeZone_variableToArray "$functionsStartUi_timeZone_timeZoneSecondLevelV"
        functionsStartUi_timeZone_resultTimeZoneSecondLevelV=$(dialog --colors --title "\Z1\ZbTimeZone " --default-item "$functionsStartUi_timeZone_resultTimeZoneSecondLevelV" --menu "Select a TimeZone " 0 0 0 "${functionsStartUi_timeZone_argsA[@]}" --output-fd 1)
        functionsStartUi_restartCheck || continue
        break 2

    done

    if [ "$functionsStartUi_timeZone_resultTimeZoneFirstLevelV" = "$functionsStartUi_timeZone_resultTimeZoneSecondLevelV" ]; 
    then
        echo "/usr/share/zoneinfo/$functionsStartUi_timeZone_resultTimeZoneFirstLevelFileV" > $functionsCommon_chosenTimeZoneFileV
    else
        echo "/usr/share/zoneinfo/$functionsStartUi_timeZone_resultTimeZoneFirstLevelV/$functionsStartUi_timeZone_resultTimeZoneSecondLevelV" > $functionsCommon_chosenTimeZoneFileV
    fi
    functionsCommon_log "INFO" "TimeZone "
    functionsCommon_log "WARN" "$(cat $functionsCommon_chosenTimeZoneFileV)"
    return 0
}

#old yesno_f
functionsStartUi_yesNo () {
    #depends on functionsCommon_log; 

    local functionsStartUi_yesNo_resultFileV=$1
    local functionsStartUi_yesNo_defaultValueV=$(cat $functionsStartUi_yesNo_resultFileV)
    case $functionsStartUi_yesNo_defaultValueV in
        0)
            functionsStartUi_yesNo_defaultValueV="yes"
        ;;
        1)
            functionsStartUi_yesNo_defaultValueV="no"
        ;;
        *)
            functionsStartUi_yesNo_defaultValueV="no"
        ;;
    esac

    #touch $functionsStartUi_yesNo_resultFileV
    echo "" > $functionsStartUi_yesNo_resultFileV

    local functionsStartUi_yesNo_titleV=$2
    local functionsStartUi_yesNo_bodyTextV=$3

    local functionsStartUi_yesNo_returnValueV=
    local functionsStartUi_yesNo_appV=""


    dialog --colors --title "\Z1\Zb$functionsStartUi_yesNo_titleV " --default-button $functionsStartUi_yesNo_defaultValueV --yesno "\Z1$functionsStartUi_yesNo_appV\nDo you need $functionsStartUi_yesNo_bodyTextV? " 0 0
    functionsStartUi_yesNo_returnValueV=$?
    echo $functionsStartUi_yesNo_returnValueV > $functionsStartUi_yesNo_resultFileV

    appV="Please try again "
    functionsCommon_log "INFO" "$functionsStartUi_yesNo_titleV"
    functionsCommon_log "INFO" "0 means yes 1 means no 255 means interrupted "
    functionsCommon_log "WARN" "$(cat $functionsStartUi_yesNo_resultFileV)"
    return $functionsStartUi_yesNo_returnValueV

}

#for old functionsCommon software
functionsStartUi_softwareToInstall () {
    #depends on functionsCommon_log; 
    #default does not work, must create a new helper file with default values and first ask via yesNo whether to restore them
    local functionsStartUi_softwareToInstall_resultFileV=$1
    
    #touch $functionsStartUi_softwareToInstall_resultFileV
    echo "" > $functionsStartUi_softwareToInstall_resultFileV

    local functionsStartUi_softwareToInstall_titleV=$2
    local functionsStartUi_softwareToInstall_bodyTextV=$3
    local functionsStartUi_softwareToInstall_menuArgsV=$4
    local functionsStartUi_softwareToInstall_selectedV=${functionsStartUi_softwareToInstall_menuArgsV}Selected

    [[ ! -f $functionsStartUi_softwareToInstall_selectedV ]] && cp -ar $functionsStartUi_softwareToInstall_menuArgsV $functionsStartUi_softwareToInstall_selectedV

    local functionsStartUi_softwareToInstall_returnValueV=
    local functionsStartUi_softwareToInstall_appV=""
    local functionsStartUi_softwareToInstall_iV=

    
    dialog --colors --title "\Z1\Zb$functionsStartUi_softwareToInstall_titleV " --checklist "\Z1$functionsStartUi_softwareToInstall_appV\nSelect a $functionsStartUi_softwareToInstall_bodyTextV " 0 0 0 $(cat $functionsStartUi_softwareToInstall_selectedV) 2> $functionsStartUi_softwareToInstall_resultFileV
    functionsStartUi_softwareToInstall_returnValueV=$?

    functionsStartUi_softwareToInstall_appV="Please try again "

    sed -i 's/on/off/g' $functionsStartUi_softwareToInstall_selectedV

    for functionsStartUi_softwareToInstall_iV in $(cat $functionsStartUi_softwareToInstall_resultFileV) ; do
        sed -i "s/$functionsStartUi_softwareToInstall_iV $functionsStartUi_softwareToInstall_iV off/$functionsStartUi_softwareToInstall_iV $functionsStartUi_softwareToInstall_iV on/g" $functionsStartUi_softwareToInstall_selectedV
    done

    functionsCommon_log "INFO" "$functionsStartUi_softwareToInstall_titleV"
    functionsCommon_log "WARN" "$(cat $functionsStartUi_softwareToInstall_resultFileV)"

    if [[ $functionsStartUi_softwareToInstall_returnValueV == 1 ]] ; then
        cp -ar $functionsStartUi_softwareToInstall_menuArgsV $functionsStartUi_softwareToInstall_selectedV
    fi
    return $functionsStartUi_softwareToInstall_returnValueV
}


#assign variables for disks
functionsStartUi_assignDisks () {
    #depends on functionsStartUi_showDisk; functionsCommon_log; functionsCommon_uefiFileV; functionsCommon_grubFileV; functionsCommon_rootFileV; functionsStartUi_chosenDiskFileV

    #touch $functionsCommon_uefiFileV
    #touch $functionsCommon_grubFileV
    #touch $functionsCommon_rootFileV
    
    echo "" > $functionsCommon_uefiFileV
    echo "" > $functionsCommon_grubFileV
    echo "" > $functionsCommon_rootFileV

    if [[ "$(cat ${functionsStartUi_chosenDiskFileV})" == *"nvme"* ]] ; 
    then
        functionsCommon_log "INFO" "Your disk is an nvme "
        echo $(cat $functionsStartUi_chosenDiskFileV)p1 > $functionsCommon_uefiFileV
        echo $(cat $functionsStartUi_chosenDiskFileV)p2 > $functionsCommon_grubFileV
        echo $(cat $functionsStartUi_chosenDiskFileV)p3 > $functionsCommon_rootFileV
    else
        functionsCommon_log "INFO" "Your disk is not an nvme "
        echo $(cat $functionsStartUi_chosenDiskFileV)1 > $functionsCommon_uefiFileV
        echo $(cat $functionsStartUi_chosenDiskFileV)2 > $functionsCommon_grubFileV
        echo $(cat $functionsStartUi_chosenDiskFileV)3 > $functionsCommon_rootFileV
    fi
    functionsCommon_log "WARN" "$(cat $functionsCommon_uefiFileV)"
    functionsCommon_log "WARN" "$(cat $functionsCommon_grubFileV)"
    functionsCommon_log "WARN" "$(cat $functionsCommon_rootFileV)"
    
}

functionsStartUi_choses () {
    #depends on functionsCommon_log; functionsStartUi_messagebox; functionsStartUi_inputbox; functionsStartUi_restartCheck; functionsStartUi_passwordBox; functionsStartUi_menuSelect; functionsStartUi_timeZone; 

    while true ; do

        #initial message
        functionsStartUi_messageBox "Welcome to simple Void Linux install script." "This script will install Void Linux. \nWhat the script will do: \n1. Encrypted boot and root, \n2. /boot/efi in fat32 unencrypted, 512 Mb reccomanded, \n3. /boot in ext4 encrypted in LUKS1, 4 Gb reccomended, \n4. / in btrfs encrypted in LUKS2, \n5. / key in initrfs for typing password once, \n6. no LVM, instead there are btrfs sobvolumes, \n7. swapfile in a btrfs subvolume with no COW and no compression, 16 Gb, \n8. subvolumes schemes are copyed by OpenSuse configuration, in that way nothing unnecessary will be included into snapshots, \n9. fstab optimizations for ssds and compression, \n10. snapper configured and first snapshoot taked, \n11. Void Linux musl 64bit only, \n12. free software only with a lxd container for glibc and non free software, \n13. s6 init instead of runit on the main system and on the container, \n14. the best of all programs (like chrony for ntp, ecc.), \n15. KDE Plasma desktop with xorg, \n16. ClamAV and ClamTK for antivirus software, \n17. a lot of software, all you will need! \n18. flatpak on userspace to install necessary proprietary software (like firefox DRM to what Netflix or Amazon Prime Video), \nMake sure to follow the instructions, all in detailed and well documented, if you want to modify it just do it! \nMake shure that you are in the right directory (cd gab-void-linux-installer)."

        #input hostname
        functionsStartUi_inputBox $functionsCommon_chosenHostNameFileV "Hostname " "Give me the Hostname "
        functionsStartUi_restartCheck || continue 1

        #input Encryption password
        functionsStartUi_passwordBox $functionsCommon_chosenEncryptPasswordFileV "Encrypt " "Decryption "
        functionsStartUi_restartCheck || continue 1

        #input swapfile size
        functionsStartUi_inputBox $functionsCommon_chosenSwapFileSizeV "Swapfile size " "Give me the size of the swapfile (it must be a number, not a decimal, expessed in Gb) "
        functionsStartUi_restartCheck || continue 1
        #todo check input with if return 0 else return 1

        #mirror benchmark
        #functionsStartUi_mirrorsBenchmark 
        #functionsStartUi_menuSelect $functionsCommon_chosenRepositoryFileV "Benchmark repository " "Repository. Note that the best for you is $(cat $functionsStartUi_benchmarkMirrorsV | awk '{print $1}') " "$functionsStartUi_benchmarkMirrorsV"

        #input repos
        functionsStartUi_inputBox $functionsCommon_chosenRepositoryFileV "Repository " "Give me the ip of you machine for the repository (or leave the selected one) (example alpha.de.repo.voidlinux.org) "
        functionsStartUi_restartCheck || continue 1

        #input Username
        functionsStartUi_inputBox $functionsCommon_chosenUserNameFileV "User" "Give me the Username "
        functionsStartUi_restartCheck || continue 1

        #input user and root password
        functionsStartUi_passwordBox $functionsCommon_chosenUserPasswordFileV "User and root password " "User and root password "
        functionsStartUi_restartCheck || continue 1

        #input language 
        local functionsStartUi_choses_languageListV=$(ls -d /usr/share/locale/*/ | sed "s/\/usr\/share\/locale\///g" | sed 's/.$//')
        functionsStartUi_menuSelect $functionsCommon_chosenLanguageFileV "Language " "Language " "$functionsStartUi_choses_languageListV"
        functionsStartUi_restartCheck || continue 1

        #input keymap 
        local functionsStartUi_choses_keymapsListV=$(find /usr/share/kbd/keymaps/ -type f -iname "*.map.gz" | sed "s/.*\///; s/\.map.gz//" | sort)
        functionsStartUi_menuSelect $functionsCommon_chosenKeymapFileV "Keymap " "Keymap " "$functionsStartUi_choses_keymapsListV"
        functionsStartUi_restartCheck || continue 1

        #input timezone
        functionsStartUi_timeZone
        functionsStartUi_restartCheck || continue 1

    ##start installing software

        #input video drivers
        local functionsStartUi_choses_driversV=$'amd\nintel\nnvidia\nnone'
        functionsStartUi_menuSelect $functionsCommon_chosenDriverFileV "Video drivers " "Video drivers " "$functionsStartUi_choses_driversV"
        functionsStartUi_restartCheck || continue 1

        #input editor 
        local functionsStartUi_choses_editorsV=$'nano\nvim\nemacs\nne\nneovim\ntilde'
        functionsStartUi_menuSelect $functionsCommon_chosenEditorFileV "Editor " "Editor " "$functionsStartUi_choses_editorsV"
        functionsStartUi_restartCheck || continue 1

        #input shell 
        local functionsStartUi_choses_shellsV=$'zsh\nbash\nksh\nyash'
        functionsStartUi_menuSelect $functionsCommon_chosenShellFileV "Shell " "Shell " "$functionsStartUi_choses_shellsV"
        functionsStartUi_restartCheck || continue 1

        #input desktop environment 
        local functionsStartUi_choses_desktopEnvironmentsV=$'kde\ngnome\nxfce\nmate\ncinnamon\nlxde\nlxqt\nenlightenment\nnone'
        functionsStartUi_menuSelect $functionsCommon_chosenDesktopEnvironmentFileV "Desktop Environment " "Desktop Environment " "$functionsStartUi_choses_desktopEnvironmentsV"
        functionsStartUi_restartCheck || continue 1

        #input bluethoot
        functionsStartUi_yesNo $functionsCommon_bluethootFileV "Bluethoot " "Bluethoot " 

        #input printers
        functionsStartUi_yesNo $functionsCommon_printersFileV "Printers " "Printers " 

        #input virtualization
        functionsStartUi_yesNo $functionsCommon_virtualizationFileV "Virtualization " "Virtualization (qemu, kvm, virt-manager) " 

        #input lxd
        functionsStartUi_yesNo $functionsCommon_lxdFileV "Lxd " "Containers (lxd) " 

        #input samba
        functionsStartUi_yesNo $functionsCommon_sambaFileV "Samba " "samba " 

        #input clamav
        functionsStartUi_yesNo $functionsCommon_clamavFileV "ClamAV " "clamav " 

        #input other Software
        functionsStartUi_softwareToInstall $functionsCommon_chosenOtherSoftwareFileV "Other Software " "Other Software " "$functionsStartUi_otherSoftwareFileV"
        functionsStartUi_restartCheck || continue 1

    ##finish installing software

        #input disk 
        local functionsStartUi_choses_showDiskV=$(lsblk -dp | grep -o '^/dev[^ ]*' | grep -e $*vd. -e $*sd. -e $*nvme)
        functionsStartUi_menuSelect $functionsStartUi_chosenDiskFileV "Disk " "Disk " "$functionsStartUi_choses_showDiskV"
        functionsStartUi_restartCheck || continue 1

        functionsStartUi_assignDisks

        #ultimate check
        echo "" > $functionsStartUi_ultimateCheckFileV
        functionsStartUi_yesNo $functionsStartUi_ultimateCheckFileV "Ultimate check " "this configuration (if no then start to the begin), carefull this is the last choise, if you chose YES your disk will be \Z1ERASED and all data will be lost "
        functionsStartUi_restartCheck || continue 1


        break
    done
}
