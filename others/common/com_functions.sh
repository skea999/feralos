#!/bin/bash
#source /scriptFilesD/othersD/othersDistributionSpecificThingsF.sh

com_fun_chosenRepositoryFileV=/scriptFilesD/resultsD/chosenRepositoryF 
com_fun_chosenHostNameFileV=/scriptFilesD/resultsD/chosenHostNameF 
com_fun_chosenEncryptPasswordFileV=/scriptFilesD/resultsD/chosenEncryptPasswordF 
com_fun_chosenSwapFileSizeV=/scriptFilesD/resultsD/chosenSwapSizeF
com_fun_chosenUserNameFileV=/scriptFilesD/resultsD/chosenUserNameF 
com_fun_chosenUserPasswordFileV=/scriptFilesD/resultsD/chosenUserPasswordF 
com_fun_chosenLanguageFileV=/scriptFilesD/resultsD/chosenLanguageF 
com_fun_chosenKeymapFileV=/scriptFilesD/resultsD/chosenKeymapF 
com_fun_chosenTimeZoneFileV=/scriptFilesD/resultsD/chosenTimeZoneF 
com_fun_chosenDriverFileV=/scriptFilesD/resultsD/chosenDriverF 
com_fun_chosenEditorFileV=/scriptFilesD/resultsD/chosenEditorF 
com_fun_chosenShellFileV=/scriptFilesD/resultsD/chosenShellF 
com_fun_chosenDesktopEnvironmentFileV=/scriptFilesD/resultsD/chosenDesktopEnvironmentF 
com_fun_bluethootFileV=/scriptFilesD/resultsD/bluethootF 
com_fun_printersFileV=/scriptFilesD/resultsD/printersF 
com_fun_virtualizationFileV=/scriptFilesD/resultsD/virtualizationF 
com_fun_lxdFileV=/scriptFilesD/resultsD/lxdF 
com_fun_sambaFileV=/scriptFilesD/resultsD/sambaF 
com_fun_clamavFileV=/scriptFilesD/resultsD/clamavF 
com_fun_chosenOtherSoftwareFileV=/scriptFilesD/resultsD/chosenOtherSoftwareF 
com_fun_uefiFileV=/scriptFilesD/resultsD/uefiF 
com_fun_grubFileV=/scriptFilesD/resultsD/grubF 
com_fun_rootFileV=/scriptFilesD/resultsD/rootF 

com_fun_configurationLogFilePathV=/scriptFilesD/installLogF.log
com_fun_logFailedCommandsFileV=/scriptFilesD/logFailedCommandsF.log




#print a colored message, based on an argument taked as imput ("INFO" "ERROR" "WARNING"), like: com_fun_log "INFO" "commandToExecute"
com_fun_log () {
    local com_fun_log_typeV=$1
    local com_fun_log_textV=$2
    local com_fun_log_noColorV='\033[0m'
    local com_fun_log_redColorV='\033[0;31m'
    local com_fun_log_yellowColorV='\033[1;33m'
    local com_fun_log_blueColorV='\033[1;34m'
    local com_fun_log_colorV=$com_fun_log_noColorV


    case "$com_fun_log_typeV" in
        "INFO")
            com_fun_log_colorV=$com_fun_log_blueColorV
        ;;
        "WARN")
            com_fun_log_colorV=$com_fun_log_yellowColorV
        ;;
        "ERROR")
            com_fun_log_colorV=$com_fun_log_redColorV
        ;;
    esac

    echo $com_fun_log_textV >> $com_fun_configurationLogFilePathV

    echo
    echo -e "${com_fun_log_colorV}$com_fun_log_textV${com_fun_log_noColorV}"
    echo
}

#Put a string at the end of a file
com_fun_stringToFile () {
    #depends on com_fun_log

    local com_fun_stringToFile_stringV=$1
    local com_fun_stringToFile_fileV=$2
    #local com_fun_stringToFile_ecapedStringV=$(printf '%s\n' "$com_fun_stringToFile_stringV" | sed -e 's/[\/&]/\\&/g')


    com_fun_log "INFO" "writing: $com_fun_stringToFile_stringV in $com_fun_stringToFile_fileV"
    echo "$com_fun_stringToFile_stringV" >> $com_fun_stringToFile_fileV
    #put an input if you want to dysplay the file
    #com_fun_log "WARN" "$(cat $com_fun_stringToFile_fileV) "
    echo
}

#old verify_f
com_fun_retryIfCommandFails () {
    #depends on com_fun_log; com_fun_stringToFile

    local com_fun_retryIfCommandFails_commandToRunV=$1
    local -i com_fun_retryIfCommandFails_returnValueV
    local -i com_fun_retryIfCommandFails_numberRetryV=3
    local -i com_fun_retryIfCommandFails_iV=1


    com_fun_log "WARN" "Executing $com_fun_retryIfCommandFails_commandToRunV ... " $com_fun_configurationLogFilePathV
    while [[ "$com_fun_retryIfCommandFails_returnValueV" !=  "0" ]] && [[ $com_fun_retryIfCommandFails_iV -le $com_fun_retryIfCommandFails_numberRetryV ]] ; do
        echo
        eval "$com_fun_retryIfCommandFails_commandToRunV"
        com_fun_retryIfCommandFails_returnValueV=$?
        echo
        if [ $com_fun_retryIfCommandFails_returnValueV != 0 ]; then
            com_fun_log "ERROR" "$com_fun_retryIfCommandFails_commandToRunV"
            com_fun_log "WARN" "Retrying $com_fun_retryIfCommandFails_iV time... "
            read -t 5 -p "Press enter to retry. I am going to wait for 5 seconds only ... "
        fi
        if [ $com_fun_retryIfCommandFails_iV = $com_fun_retryIfCommandFails_numberRetryV ]; then
            dialog --colors --default-button "yes" --yesno "\nDo you want to retry \n\Z1$com_fun_retryIfCommandFails_commandToRunV \n that gived an error $com_fun_retryIfCommandFails_iV times? " 0 0
            if [ $? = 1 ]; then
                com_fun_stringToFile "Command $com_fun_retryIfCommandFails_commandToRunV gived an error, pls fix it. " $com_fun_logFailedCommandsFileV
                echo
            else
                com_fun_retryIfCommandFails_iV=0
            fi
        fi
        ((com_fun_retryIfCommandFails_iV=com_fun_retryIfCommandFails_iV+1))
        #echo $iV
    done
}

#it works now, need to be integrated
com_fun_backupRestoreIFS () {
    local com_fun_backupRestoreIfs_savedIfsV=$IFS
    local com_fun_backupRestoreIFS_commandToRunV=$1
    IFS=$'\n'
    eval "$com_fun_backupRestoreIFS_commandToRunV"
    IFS=$com_fun_backupRestoreIfs_savedIfsV
}
