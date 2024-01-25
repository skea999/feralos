#!/bin/bash
#source /scriptFilesD/othersD/othersDistributionSpecificThingsF.sh

functionsCommon_chosenRepositoryFileV=/scriptFilesD/resultsD/chosenRepositoryF 
functionsCommon_chosenHostNameFileV=/scriptFilesD/resultsD/chosenHostNameF 
functionsCommon_chosenEncryptPasswordFileV=/scriptFilesD/resultsD/chosenEncryptPasswordF 
functionsCommon_chosenSwapFileSizeV=/scriptFilesD/resultsD/chosenSwapSizeF
functionsCommon_chosenUserNameFileV=/scriptFilesD/resultsD/chosenUserNameF 
functionsCommon_chosenUserPasswordFileV=/scriptFilesD/resultsD/chosenUserPasswordF 
functionsCommon_chosenLanguageFileV=/scriptFilesD/resultsD/chosenLanguageF 
functionsCommon_chosenKeymapFileV=/scriptFilesD/resultsD/chosenKeymapF 
functionsCommon_chosenTimeZoneFileV=/scriptFilesD/resultsD/chosenTimeZoneF 
functionsCommon_chosenDriverFileV=/scriptFilesD/resultsD/chosenDriverF 
functionsCommon_chosenEditorFileV=/scriptFilesD/resultsD/chosenEditorF 
functionsCommon_chosenShellFileV=/scriptFilesD/resultsD/chosenShellF 
functionsCommon_chosenDesktopEnvironmentFileV=/scriptFilesD/resultsD/chosenDesktopEnvironmentF 
functionsCommon_bluethootFileV=/scriptFilesD/resultsD/bluethootF 
functionsCommon_printersFileV=/scriptFilesD/resultsD/printersF 
functionsCommon_virtualizationFileV=/scriptFilesD/resultsD/virtualizationF 
functionsCommon_lxdFileV=/scriptFilesD/resultsD/lxdF 
functionsCommon_sambaFileV=/scriptFilesD/resultsD/sambaF 
functionsCommon_clamavFileV=/scriptFilesD/resultsD/clamavF 
functionsCommon_chosenOtherSoftwareFileV=/scriptFilesD/resultsD/chosenOtherSoftwareF 
functionsCommon_uefiFileV=/scriptFilesD/resultsD/uefiF 
functionsCommon_grubFileV=/scriptFilesD/resultsD/grubF 
functionsCommon_rootFileV=/scriptFilesD/resultsD/rootF 

functionsCommon_configurationLogFilePathV=/scriptFilesD/installLogF.log
functionsCommon_logFailedCommandsFileV=/scriptFilesD/logFailedCommandsF.log




#print a colored message, based on an argument taked as imput ("INFO" "ERROR" "WARNING"), like: functionsCommon_log "INFO" "commandToExecute"
functionsCommon_log () {
    local functionsCommon_log_typeV=$1
    local functionsCommon_log_textV=$2
    local functionsCommon_log_noColorV='\033[0m'
    local functionsCommon_log_redColorV='\033[0;31m'
    local functionsCommon_log_yellowColorV='\033[1;33m'
    local functionsCommon_log_blueColorV='\033[1;34m'
    local functionsCommon_log_colorV=$functionsCommon_log_noColorV


    case "$functionsCommon_log_typeV" in
        "INFO")
            functionsCommon_log_colorV=$functionsCommon_log_blueColorV
        ;;
        "WARN")
            functionsCommon_log_colorV=$functionsCommon_log_yellowColorV
        ;;
        "ERROR")
            functionsCommon_log_colorV=$functionsCommon_log_redColorV
        ;;
    esac

    echo $functionsCommon_log_textV >> $functionsCommon_configurationLogFilePathV

    echo
    echo -e "${functionsCommon_log_colorV}$functionsCommon_log_textV${functionsCommon_log_noColorV}"
    echo
}

#Put a string at the end of a file
functionsCommon_stringToFile () {
    #depends on functionsCommon_log

    local functionsCommon_stringToFile_stringV=$1
    local functionsCommon_stringToFile_fileV=$2
    #local functionsCommon_stringToFile_ecapedStringV=$(printf '%s\n' "$functionsCommon_stringToFile_stringV" | sed -e 's/[\/&]/\\&/g')


    functionsCommon_log "INFO" "writing: $functionsCommon_stringToFile_stringV in $functionsCommon_stringToFile_fileV"
    echo "$functionsCommon_stringToFile_stringV" >> $functionsCommon_stringToFile_fileV
    #put an input if you want to dysplay the file
    #functionsCommon_log "WARN" "$(cat $functionsCommon_stringToFile_fileV) "
    echo
}

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

#it works now, need to be integrated
functionsCommon_backupRestoreIFS () {
    local functionsCommon_backupRestoreIfs_savedIfsV=$IFS
    local functionsCommon_backupRestoreIFS_commandToRunV=$1
    IFS=$'\n'
    eval "$functionsCommon_backupRestoreIFS_commandToRunV"
    IFS=$functionsCommon_backupRestoreIfs_savedIfsV
}

