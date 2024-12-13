#!/bin/bash
source /scriptFilesD/functionsD/functionsCommonF.sh
source /scriptFilesD/functionsD/functionsStartD/functionsStartUiF.sh

#format disk 
functionsStart_formatDisk () {
    #depends on functionsCommon_log

    local -i functionsStart_formatDisk_returnValueV


    functionsCommon_log "INFO" "Formatting disk $(cat $functionsStartUi_chosenDiskFileV)"

sed -e 's/\s*\([\+0-9a-zA-Z]*\).*/\1/' << EOF  | fdisk $(cat $functionsStartUi_chosenDiskFileV)
g# create new GPT partition
n# add new partition
1# partition number
# default - first sector 
+512M# partition size
n# add new partition
2# partition number
# default - first sector 
+4096M# partition size
n# add new partition
3# partition number
# default - first sector 
# partition size
t# change partition type
1# partition number
1# Linux filesystem
t# change partition type
2# partition number
20# Linux filesystem
t# change partition type
3# partition number
20# Linux filesystem
w# write partition table and exit
EOF
    functionsStart_formatDisk_returnValueV=$?

    functionsCommon_log "WARN" "$(fdisk -l) "

    return $functionsStart_formatDisk_returnValueV
}

#encrypt and open partitions
functionsStart_cryptSetup () {
    #depends on functionsCommon_log; functionsCommon_retryIfCommandFails

    local functionsStart_cryptSetup_typeV=$1
    local functionsStart_cryptSetup_diskV=$2
    local functionsStart_cryptSetup_diskNameV=$3
    #local -i functionsStart_cryptSetup_returnValueV

    functionsCommon_log "INFO" "$functionsStart_cryptSetup_diskNameV "

    functionsCommon_retryIfCommandFails "cat $functionsCommon_chosenEncryptPasswordFileV | cryptsetup luksFormat --type $functionsStart_cryptSetup_typeV $(cat $functionsStart_cryptSetup_diskV)"

    functionsCommon_retryIfCommandFails "cat $functionsCommon_chosenEncryptPasswordFileV | cryptsetup luksOpen $(cat $functionsStart_cryptSetup_diskV) luks-$(blkid -o value -s UUID $(cat $functionsStart_cryptSetup_diskV))"

}

functionsStart_defaultDiskConfig () {
    #depends on functionsCommon_log;
    #local -i functionsStart_cdefaultDiskConfig_returnValueV
    functionsCommon_log "INFO" "Creating file systems "
    echo
    functionsCommon_log "INFO" "Grub "
    functionsCommon_retryIfCommandFails "mkfs.ext4 -L grub /dev/mapper/luks-$(blkid -o value -s UUID $(cat ${functionsCommon_grubFileV}))"
    functionsCommon_log "INFO" "Root "
    echo
    functionsCommon_log "INFO" "Creating volume group "
    functionsCommon_retryIfCommandFails "vgcreate voidvm /dev/mapper/luks-$(blkid -o value -s UUID $(cat ${functionsCommon_rootFileV}))"
    functionsCommon_log "INFO" "Creating logical volume of voidvm "
    functionsCommon_retryIfCommandFails "lvcreate --name root -l 100%FREE voidvm"
    functionsCommon_log "INFO" "Creating file systems for root "
    functionsCommon_retryIfCommandFails "mkfs.btrfs -L root /dev/voidvm/root"
    functionsCommon_log "INFO" "Efi "
    functionsCommon_retryIfCommandFails "mkfs.vfat -F 32 $(cat ${functionsCommon_uefiFileV})"
}

functionsStart_createSubvolumes () {
    #depends on functionsCommon_log; functionsCommon_retryIfCommandFails
    local functionsStart_createSubvolumes_iV

    functionsCommon_log "INFO" "Creating subvolumes "
    echo
    functionsCommon_retryIfCommandFails "mount /dev/voidvm/root /mnt"

    #creating sobvolumes #fare dentro un ciclo for
    for functionsStart_createSubvolumes_iV in "/mnt/@" "/mnt/@swapfile" "/mnt/@home" "/mnt/@opt" "/mnt/@root" "/mnt/@srv" "/mnt/@tmp" "/mnt/@var" "/mnt/@var-lib-ex" "/mnt/@var-lib-flatpak" "/mnt/@var-lib-libvirt" "/mnt/@var-lib-lxd" "/mnt/@var-lib-containerd" "/mnt/@var-lib-docker" "/mnt/@var-cache" "/mnt/@var-log" "/mnt/@var-opt" "/mnt/@var-spool" "/mnt/@var-tmp" "/mnt/@usr-local";
    do
        functionsCommon_log "INFO" "Creating $functionsStart_createSubvolumes_iV"
        functionsCommon_retryIfCommandFails "btrfs subvolume create $functionsStart_createSubvolumes_iV "
    done


    functionsCommon_log "WARN" "$(btrfs subvolume list /mnt)"
    functionsCommon_retryIfCommandFails "$(umount /mnt)"
}

functionsStart_mountPartitions () {
    #depends on functionsCommon_log; functionsCommon_retryIfCommandFails
    local functionsStart_mountPartitions_iV
    local functionsStart_mountPartitions_jV

    for functionsStart_mountPartitions_iV in "" "home" "opt" "root" "var" "var-lib-flatpak" "var-lib-libvirt" "var-lib-lxd" "var-lib-containerd" "var-lib-docker" "var-cache" "var-opt" "usr-local";
    do
        functionsCommon_log "INFO" "mounting ${functionsStart_mountPartitions_iV}"
        functionsStart_mountPartitions_jV="$(echo $functionsStart_mountPartitions_iV | sed "s/-/\//g")"
        functionsCommon_retryIfCommandFails "mkdir -p /mnt/${functionsStart_mountPartitions_jV}"
        functionsCommon_retryIfCommandFails "mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@${functionsStart_mountPartitions_iV} /dev/voidvm/root /mnt/${functionsStart_mountPartitions_jV}"
    done

    for functionsStart_mountPartitions_iV in "srv" "var-lib-ex" "var-log" "var-spool" "var-tmp";
    do
        functionsCommon_log "INFO" "mounting ${functionsStart_mountPartitions_iV}"
        functionsStart_mountPartitions_jV="$(echo $functionsStart_mountPartitions_iV | sed "s/-/\//g")"
        functionsCommon_retryIfCommandFails "mkdir -p /mnt/${functionsStart_mountPartitions_jV}"
        functionsCommon_retryIfCommandFails "mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@${functionsStart_mountPartitions_iV} /dev/voidvm/root /mnt/${functionsStart_mountPartitions_jV}"
        functionsCommon_retryIfCommandFails "chattr +C /mnt/${functionsStart_mountPartitions_jV}"
        #functionsCommon_retryIfCommandFails "btrfs property set /mnt/${functionsStart_mountPartitions_jV} compression none"
    done


    functionsCommon_log "INFO" "mounting swapfile"
    functionsCommon_retryIfCommandFails "mkdir -p /mnt/swapfile"
    functionsCommon_retryIfCommandFails "mount -t btrfs -o compress=no,autodefrag,ssd,nodatacow,subvol=@swapfile /dev/voidvm/root /mnt/swapfile"
    functionsCommon_retryIfCommandFails "chattr +C /mnt/swapfile"
    #functionsCommon_retryIfCommandFails "btrfs property set /mnt/swapfile compression none"

    functionsCommon_log "INFO" "mounting tmp"
    functionsCommon_retryIfCommandFails "mkdir -p /mnt/tmp"
    functionsCommon_retryIfCommandFails "mount -t btrfs -o compress=zstd:3,autodefrag,ssd,noexec,nosuid,subvol=@tmp /dev/voidvm/root /mnt/tmp"


    functionsCommon_log "INFO" "mounting boot"
    functionsCommon_retryIfCommandFails "mkdir -p /mnt/boot"
    functionsCommon_retryIfCommandFails "mount /dev/mapper/luks-$(blkid -o value -s UUID $(cat $functionsCommon_grubFileV)) /mnt/boot"

    functionsCommon_log "INFO" "mounting efi"
    functionsCommon_retryIfCommandFails "mkdir -p /mnt/boot/efi"
    functionsCommon_retryIfCommandFails "mount $(cat $functionsCommon_uefiFileV) /mnt/boot/efi"

    functionsCommon_log "WARN" "$(mount | grep mnt)"
}

functionsStart_mountSystem () {
    #depends on functionsCommon_log; functionsCommon_retryIfCommandFails
    local functionsStart_mountSystem_iV

    functionsCommon_log "INFO" "Mounting sys "
    functionsCommon_retryIfCommandFails "mkdir -p /mnt/sys"
    functionsCommon_retryIfCommandFails "mount --rbind /sys /mnt/sys && mount --make-rslave /mnt/sys"

    functionsCommon_log "INFO" "Mounting dev "
    functionsCommon_retryIfCommandFails "mkdir -p /mnt/dev"
    functionsCommon_retryIfCommandFails "mount --rbind /dev /mnt/dev && mount --make-rslave /mnt/dev"

    functionsCommon_log "INFO" "Mounting proc "
    functionsCommon_retryIfCommandFails "mkdir -p /mnt/proc"
    functionsCommon_retryIfCommandFails "mount --rbind /proc /mnt/proc && mount --make-rslave /mnt/proc"

    for functionsStart_mountSystem_iV in "/mnt/sys" "/mnt/dev" "/mnt/proc";
    do
        functionsCommon_log "WARN" "$(mount | grep $functionsStart_mountSystem_iV)"
    done
}

functionsStart_choiceInChroot () {
    #depends on functionsCommon_log; functionsCommon_retryIfCommandFails
    functionsCommon_log "INFO" "Coping variables in /mnt "

    functionsCommon_retryIfCommandFails "mkdir -p /mnt/scriptFilesD/"
    functionsCommon_retryIfCommandFails "cp -ar /scriptFilesD/* /mnt/scriptFilesD/"

    #cp -r ./others /mnt/scriptv/others i want to copy it first and then functionsStart the script
    functionsCommon_retryIfCommandFails "chmod -R +x /mnt/scriptFilesD/"

    functionsCommon_log "WARN" "$(ls -la /mnt/scriptFilesD)"
}