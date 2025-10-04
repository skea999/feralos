#! /bin/bash

#depends on functionsCommonF.sh; functionsStartUiF.sh; functionsStartF.sh; 

#Check the actual user that is running the script and exit if it is not root 
#functionsins_checkUser
    echo
    echo "Checking if the script is executed by the root user "
    echo
    [ "$(id -u)" != "0" ] && echo "This script must be run from root. " && exit 1  
    echo

ins_logCommand () {
    ins_log_commandV=$1
    ins_log_pathV=$2
    ins_log_executeV="${ins_log_commandV} ${ins_log_pathV} "
    echo "$ins_log_executeV"
    eval "$ins_log_executeV"
    echo $? >> /scriptFilesD/installLogF.log
    echo "$ins_log_executeV" >> /scriptFilesD/installLogF.log
    ls -lah "$ins_log_pathV"
    echo "$(ls -lah $ins_log_pathV)" >> /scriptFilesD/installLogF.log
}

#make a directory to store used variables
ins_logCommand "mkdir -p" "/scriptFilesD"

ins_logCommand "mkdir -p" "/scriptFilesD/resultsD"

#copy thing in /scriptFilesD
ins_logCommand "cp -ar ./*" "/scriptFilesD"

ins_logCommand "chmod -R +x" "/scriptFilesD"

#source desired function

ins_logCommand "source" "/scriptFilesD/functionsD/functionsCommonF.sh"

ins_logCommand "touch" "$functionsCommon_chosenOtherSoftwareFileV"

ins_logCommand "source" "/scriptFilesD/othersD/othersDistributionSpecificThingsF.sh"

ins_logCommand "source" "/scriptFilesD/functionsD/functionsStartD/functionsStartF.sh"
ins_logCommand "source" "/scriptFilesD/functionsD/functionsStartD/functionsStartUiF.sh"


#clear
#Initial message
functionsCommon_log "INFO" "Please wait the preparation of the installer "

#connect to internet #need future implementation

#setting up DNS
functionsCommon_retryIfCommandFails "echo > \"/etc/resolv.conf\""
functionsCommon_stringToFile "nameserver 208.67.222.222" "/etc/resolv.conf"
functionsCommon_stringToFile "nameserver 208.67.220.220" "/etc/resolv.conf"

#update package manager
othersDistributionSpecificThings_updatePackageManager

#install dependencies
othersDistributionSpecificThings_installPackage "$othersDistributionSpecificThings_initialDependenciesV "

#create dialog config
functionsStartUi_init
functionsStartUi_choses

#inform user
functionsCommon_log "INFO" "We are making things done thank you for your patience "


#format disk #need future implementations
functionsCommon_retryIfCommandFails "functionsins_formatDisk"

#crypting and opening partitions
functionsCommon_log "INFO" "Encrypting and opening partitions "
functionsins_cryptSetup "luks1" "$functionsCommon_grubFileV" "Grub"
functionsins_cryptSetup "luks2" "$functionsCommon_rootFileV" "Root"

#creating file systems
functionsins_defaultDiskConfig

#creating sobvolumes
functionsins_createSubvolumes

#mounting file systems
functionsins_mountPartitions

#prepairing for chroot
functionsins_mountSystem

#setupd base-system
othersDistributionSpecificThings_baseSystem

#taking variables to chroot
functionsins_choiceInChroot

#entering chroot
functionsCommon_log "INFO" "Entering chroot "
chroot /mnt /bin/bash /scriptFilesD/othersD/othersChrootF.sh

functionsCommon_log "INFO" "Ultimate message "
functionsStartUi_messageBox "Ska installer " "Installed successfull\nIf all is gone well umount /mnt with:\nsudo umount -R /mnt\nAnd than reboot\nRemember to execute other sccripts once system will boot \n\nThank you for using this script "
