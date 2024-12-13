#!/bin/bash
source /scriptFilesD/functionsD/functionsCommonF.sh
source /scriptFilesD/othersD/othersDistributionSpecificThingsF.sh

functionsStartUi_benchmarkMirrorsV=
functionsStartUi_otherSoftwareFileV=/scriptFilesD/othersD/othersSoftwareToInstallF 
functionsStartUi_chosenDiskFileV=/scriptFilesD/chosenDiskF 
functionsStartUi_ultimateCheckFileV=/scriptFilesD/ultimateCheckF


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
