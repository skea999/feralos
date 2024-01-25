#! /bin/bash

#depends on functionsCommonF.sh; functionsStartUiF.sh; functionsStartF.sh; 

#Check the actual user that is running the script and exit if it is not root 
#functionsStart_checkUser
    echo
    echo "Checking if the script is executed by the root user "
    echo
    [ "$(id -u)" != "0" ] && echo "This script must be run from root. " && exit 1  
    echo

start_logCommand () {
    start_logCommand_commandV=$1
    start_logCommand_pathV=$2
    start_logCommand_executeV="${start_logCommand_commandV} ${start_logCommand_pathV} "
    echo "$start_logCommand_executeV"
    eval "$start_logCommand_executeV"
    echo $? >> /scriptFilesD/installLogF.log
    echo "$start_logCommand_executeV" >> /scriptFilesD/installLogF.log
    ls -lah "$start_logCommand_pathV"
    echo "$(ls -lah $start_logCommand_pathV)" >> /scriptFilesD/installLogF.log
}

#make a directory to store used variables
start_logCommand "mkdir -p" "/scriptFilesD"

start_logCommand "mkdir -p" "/scriptFilesD/resultsD"

#copy thing in /scriptFilesD
start_logCommand "cp -ar ./*" "/scriptFilesD"

start_logCommand "chmod -R +x" "/scriptFilesD"

#source desired function

start_logCommand "source" "/scriptFilesD/functionsD/functionsCommonF.sh"

start_logCommand "touch" "$functionsCommon_chosenOtherSoftwareFileV"

start_logCommand "source" "/scriptFilesD/othersD/othersDistributionSpecificThingsF.sh"

start_logCommand "source" "/scriptFilesD/functionsD/functionsStartD/functionsStartF.sh"
start_logCommand "source" "/scriptFilesD/functionsD/functionsStartD/functionsStartUiF.sh"


#clear
#Initial message
functionsCommon_log "INFO" "Please wait the preparation of the installer "

#connect to internet #need future implementation

#setting up DNS
functionsCommon_retryIfCommandFails "echo > \"/etc/resolv.conf\""
functionsCommon_stringToFile "nameserver 8.8.8.8" "/etc/resolv.conf"

#update package manager
othersDistributionSpecificThings_updatePackageManager

#install dependencies ### don't know if it works
othersDistributionSpecificThings_installPackage "$othersDistributionSpecificThings_initialDependenciesV "

#create dialog config
functionsStartUi_init
functionsStartUi_choses

#inform user
functionsCommon_log "INFO" "We are making things done thank you for your patience "


#format disk #need future implementations
functionsCommon_retryIfCommandFails "functionsStart_formatDisk"

#crypting and opening partitions
functionsCommon_log "INFO" "Encrypting and opening partitions "
functionsStart_cryptSetup "luks1" "$functionsCommon_grubFileV" "Grub"
functionsStart_cryptSetup "luks2" "$functionsCommon_rootFileV" "Root"

#creating file systems
functionsStart_defaultDiskConfig

#creating sobvolumes
functionsStart_createSubvolumes

#mounting file systems
functionsStart_mountPartitions

#prepairing for chroot
functionsStart_mountSystem

#setupd base-system
othersDistributionSpecificThings_baseSystem

#taking variables to chroot
functionsStart_choiceInChroot

#entering chroot
functionsCommon_log "INFO" "Entering chroot "
chroot /mnt /bin/bash /scriptFilesD/othersD/othersChrootF.sh

functionsCommon_log "INFO" "Ultimate message "
functionsStartUi_messageBox "Ska installer " "Installed successfull\nIf all is gone well umount /mnt with:\nsudo umount -R /mnt\nAnd than reboot\nRemember to execute other sccripts once system will boot \n\nThank you for using this script "

