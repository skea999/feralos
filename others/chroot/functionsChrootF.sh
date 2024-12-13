#!/bin/bash

source /scriptFilesD/functionsD/functionsCommonF.sh
source /scriptFilesD/othersD/othersDistributionSpecificThingsF.sh
source /scriptFilesD/functionsD/functionsChrootF.sh




functionsChroot_checkPackages () {
    
    #video drivers
    if [ "$functionsChrootHardwareDetenction_detectGPUVendors" = *"amd"* ]; then
        othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesAmdV} "
    fi
    if [ "$functionsChrootHardwareDetenction_detectGPUVendors" = *"intel"* ]; then
        othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesIntelV} "
    fi
    if [ "$functionsChrootHardwareDetenction_detectGPUVendors" = *"nvidia"* ]; then
        othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesNvidiaV} "
    fi

    # #video drivers
    # if [ "$(cat "$functionsCommon_chosenDriverFileV")" = *"amd"* ]; then
    #     othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesAmdV} "
    # fi
    # if [ "$(cat "$functionsCommon_chosenDriverFileV")" = *"intel"* ]; then
    #     othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesIntelV} "
    # fi
    # if [ "$(cat "$functionsCommon_chosenDriverFileV")" = *"nvidia"* ]; then
    #     othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesNvidiaV} "
    # fi
    
    #editor
    case "$(cat "$functionsCommon_chosenEditorFileV")" in
        nano)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesNanoV} "
        ;;
        vim)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesVimV} "
        ;;
        ne)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesNeV} "
        ;;
        neovim)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesNeovimV} "
        ;;
        tilde)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesTildeV} "
        ;;
    esac

    #shell
    case "$(cat "$functionsCommon_chosenShellFileV")" in
        zsh)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesZshV} "
        ;;
        bash)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesBashV} "
        ;;
        ksh)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesKshV} "
        ;;
        yash)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesYashV} "
        ;;
    esac

    #desktop environment 
    case "$(cat "$functionsCommon_chosenDesktopEnvironmentFileV")" in
        kde)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesKdeV} "
        ;;
        gnome)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesGnomeV} "
        ;;
        xfce)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesXfceV} "
        ;;
        mate)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesMateV} "
        ;;
        cinnamon)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesCinnamonV} "
        ;;
        lxde)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesLxdeV} "
        ;;
        lxqt)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesLxqtV} "
        ;;
        enlightenment)
            othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesEnlightenmentV} "
        ;;
    esac

    #bluetooth
    if [ $(cat $functionsCommon_bluethootFileV) = 0 ]; then
        othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesBluezV} "
    fi
    #printers
    if [ $(cat $functionsCommon_printersFileV) = 0 ]; then
        othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesCupsEccV} "
    fi
    #virtualization
    if [ $(cat $functionsCommon_virtualizationFileV) = 0 ]; then
        othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesKvmV} "
    fi
    #lxd
    if [ $(cat $functionsCommon_lxdFileV) = 0 ]; then
        othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesLxdV} "
    fi
    #samba
    if [ $(cat $functionsCommon_sambaFileV) = 0 ]; then
        othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesSambaV} "
    fi
    #clamav
    if [ $(cat $functionsCommon_clamavFileV) = 0 ]; then
        othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_packagesToInstallV} ${othersDistributionSpecificThings_packagesClamavV} "
    fi
}

functionsChroot_rootPermission () {
    #allow members of group wheel to execute any command
    functionsCommon_log "INFO" "Allow to users of the group wheel to execute any command "
    functionsCommon_stringToFile "permit persist :wheel" "/etc/doas.conf" 
}

functionsChroot_setHostname () {
    #setting hostname 
    functionsCommon_log "INFO" "Setting hostname "
    functionsCommon_retryIfCommandFails "echo $(cat $functionsCommon_chosenHostNameFileV) > /etc/hostname"
}

functionsChroot_setLanguage () {
    #setting language 
    functionsCommon_log "INFO" "Setting locale " 
    functionsCommon_retryIfCommandFails "echo > /etc/locale.conf"
    functionsCommon_stringToFile "LANG=$(cat $functionsCommon_chosenLanguageFileV)" "/etc/locale.conf" #non so se funziona perché li c'è solo it 
}

functionsChroot_setKeymap () {
    #setting keymap 
    functionsCommon_log "INFO" "Setting keymap "
    functionsCommon_retryIfCommandFails 'sed -i "s/#KEYMAP=\"es\"/KEYMAP=\"$(cat $functionsCommon_chosenKeymapFileV)\"/g" /etc/rc.conf'
}

functionsChroot_setDns () {
    #setting DNS
    functionsCommon_log "INFO" "Setting DNS "
    functionsCommon_retryIfCommandFails "echo > /etc/resolv.conf"
    functionsCommon_stringToFile "nameserver 8.8.8.8" "/etc/resolv.conf"
    functionsCommon_stringToFile "nameserver=8.8.8.8" "/etc/resolvconf.conf"
    #change the dns to open dns instead of goole dns
}

functionsChroot_setFstab () {
    #fstab
    functionsCommon_log "INFO" "Setting fstab "
    functionsCommon_retryIfCommandFails "echo > /etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /                       btrfs    compress=zstd:3,autodefrag,ssd,subvol=@                                       0       1 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /swapfile               btrfs    compress=no,autodefrag,ssd,nodatacow,subvol=@swapfile                         0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /home                   btrfs    compress=zstd:3,autodefrag,ssd,subvol=@home                                   0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /opt                    btrfs    compress=zstd:3,autodefrag,ssd,subvol=@opt                                    0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /root                   btrfs    compress=zstd:3,autodefrag,ssd,subvol=@root                                   0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /srv                    btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@srv          0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /tmp                    btrfs    compress=zstd:3,autodefrag,ssd,noexec,nosuid,subvol=@tmp                      0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var                    btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var                                    0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var/lib/ex             btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-lib-ex   0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var/lib/flatpak        btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-flatpak                        0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var/lib/libvirt        btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-libvirt                        0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var/lib/lxd            btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-lxd                            0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var/lib/containerd     btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-containerd                     0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var/lib/docker         btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-docker                         0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var/cache              btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-cache                              0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var/log                btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-log      0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var/opt                btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-opt                                0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var/spool              btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-spool    0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /var/tmp                btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-tmp      0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/alpine/root     /usr/local              btrfs    compress=zstd:3,autodefrag,ssd,subvol=@usr-local                              0       0 " "/etc/fstab"
    functionsCommon_stringToFile "/dev/mapper/luks-$(blkid -o value -s UUID $(cat $functionsCommon_grubFileV))     /boot                   ext4     defaults                                                                      0       0 " "/etc/fstab"
    functionsCommon_stringToFile "UUID=$(blkid -o value -s UUID $(cat $functionsCommon_uefiFileV))                                            /boot/efi               vfat     defaults                                                                      0       0 " "/etc/fstab"
    functionsCommon_stringToFile "#tmpfs                                                   /tmp                    tmpfs    defaults,nosuid,nodev                                                         0       0" "/etc/fstab"
    functionsCommon_stringToFile "/swapfile/swapfile                                       none                    swap     defaults                                                                      0       0 " "/etc/fstab"
    #functionsCommon_stringToFile "/dev/mapper/luks-$(blkid -o value -s UUID $(cat /scriptv/scriptv/ROOT_V))      /.snapshots            btrfs    compress=zstd:3,autodefrag,ssd,subvol=@/.snapshots                              0       1" "/etc/fstab"
    #functionsCommon_stringToFile "/dev/mapper/luks-$(blkid -o value -s UUID $(cat /scriptv/scriptv/ROOT_V))      /home/.snapshots       btrfs    compress=zstd:3,autodefrag,ssd,subvol=@home/.snapshots                        0       1" "/etc/fstab"
}

functionsChroot_setGrub () {
    #doble check for grub installation
    functionsCommon_log "INFO" "Installing grub "

    #setting up grub to unlock the root partition
    functionsCommon_log "INFO" "Configuring grub to unlock encrypted /boot "
    functionsCommon_stringToFile "GRUB_DISABLE_OS_PROBER=false" "/etc/default/grub"
    #functionsCommon_stringToFile "GRUB_BACKGROUND=/boot/splash.png" "/etc/default/grub"
    functionsCommon_stringToFile "GRUB_ENABLE_CRYPTODISK=y" "/etc/default/grub"
    functionsCommon_retryIfCommandFails 'sed -i "s/GRUB_CMDLINE_LINUX_DEFAULT=\"loglevel=4\"/GRUB_CMDLINE_LINUX_DEFAULT=\"loglevel=0 rd.lvm.vg=alpine rd.luks.uuid=$(blkid -o value -s UUID $(cat $functionsCommon_rootFileV)) \"/g" /etc/default/grub'
    #functionsCommon_retryIfCommandFails "cp /usr/share/void-artwork/splash.png /boot" #need to be changed because it work only on void

    #installing grub
    functionsCommon_log "INFO" "Installing grub to /boot "
    functionsCommon_retryIfCommandFails 'grub-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=alpine --recheck'
}

functionsChroot_createKeyFile () {
    #creating a keyfile
    functionsCommon_log "INFO" "Creating key "
    functionsCommon_retryIfCommandFails "dd bs=1 count=256 if=/dev/urandom of=/etc/volume.key"

    #setting right permissions to the keyfile
    functionsCommon_log "INFO" "Setting right permission to keyfile "
    functionsCommon_retryIfCommandFails 'chmod 000 /etc/volume.key'
    functionsCommon_retryIfCommandFails 'chmod -R g-rwx,o-rwx /boot'
    functionsCommon_retryIfCommandFails 'chmod -R g-rwx,o-rwx /etc/volume.key'
###################################################################################################################################################################################
    #setting keyfile in the initramfs for automatic unlocking of root partiton, this way you will enter the password once
    functionsCommon_log "INFO" "Configuring dracut with key and crypttab "
    functionsCommon_retryIfCommandFails 'echo > /etc/dracut.conf.d/10-crypt.conf'
    functionsCommon_stringToFile "install_items+=\" /etc/volume.key /etc/crypttab \"" "/etc/dracut.conf.d/10-crypt.conf"
}

functionsChroot_addKey () {
    #add the keyfile to a slot for unlock the partition
    local functionsChroot_addKey_passwordV=$1
    local functionsChroot_addKey_partitionV=$2
    local functionsChroot_addKey_nameV=$3

    functionsCommon_log "INFO" "Adding a key to automatically unlock partitions "

    functionsCommon_log "INFO" "$functionsChroot_addKey_nameV "
    functionsCommon_retryIfCommandFails "cat $functionsChroot_addKey_passwordV | cryptsetup luksAddKey $(cat $functionsChroot_addKey_partitionV) /etc/volume.key"

    #setting crypttab for unlock our partitions
    functionsCommon_log "INFO" "Setting crypttab for $functionsChroot_addKey_nameV "
    functionsCommon_stringToFile "luks-$(blkid -o value -s UUID $(cat $functionsChroot_addKey_partitionV))   UUID=$(blkid -o value -s UUID $(cat $functionsChroot_addKey_partitionV))   /etc/volume.key   luks" "/etc/crypttab"
}

functionsChroot_setSwapFile () {
    #setting up a swapfile
    functionsCommon_log "INFO" "Setting swapfile "
    functionsCommon_retryIfCommandFails "chattr +C /swapfile"
    functionsCommon_log "INFO" "Disabling compression on swapfile subvolume "
    functionsCommon_retryIfCommandFails "truncate -s 0 /swapfile/swapfile"
    functionsCommon_log "INFO" "Disabling cow on swapfile subvolume "
    functionsCommon_log "INFO" "Allocating swapfile "
    functionsCommon_retryIfCommandFails "fallocate -l $(cat ${functionsCommon_chosenSwapFileSizeV})G /swapfile/swapfile"
    #functionsCommon_retryIfCommandFails "btrfs property set /swapfile/swapfile compression none"
    functionsCommon_log "INFO" "Setting right permission to swapfile "
    functionsCommon_retryIfCommandFails "chmod 600 /swapfile/swapfile"
    functionsCommon_log "INFO" "Enable swapfile "
    functionsCommon_retryIfCommandFails "mkswap /swapfile/swapfile"
}

functionsChroot_setDesktopEnvironment () {
    #Destop Environment
    case "$(cat $functionsCommon_chosenDesktopEnvironmentFileV)" in

    kde)
        othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesSddmV" "sddm"
        #export and replace /home/skikky94/.config/kdeglobals
        ;;

    gnome)
        othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesGdmV" "gdm"

        ;;

    xfce)
        othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesLightdmV" "lightdm"

        ;;

    mate)
        othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesLightdmV" "lightdm"

        ;;

    cinnamon)
        othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesLightdmV" "lightdm"

        ;;

    lxde)
        othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesLightdmV" "lightdm"

        ;;

    lxqt)
        othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesLightdmV" "lightdm"

        ;;

    enlightenment)
        othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesLightdmV" "lightdm"

        ;;

    none)
        functionsCommon_log "INFO" "No Desktop environment selected "
        ;;

    esac
}

functionsChroot_setProfile () {
    #make a decent PS1 with colors like [17:12:58] skikky94 :: gab-void-66  ➜  ~ »  
    #add git support .oh-my-zsh/plugins/gitfast/git-prompt.sh
    #oh my zsh is better, for bash there is oh my bash
    #root shell need to be the default because noone need to use it
    functionsCommon_stringToFile "if [ -f ~/.profile ]; then " "/etc/profile"
    functionsCommon_stringToFile "  . ~/.profile " "/etc/profile"
    functionsCommon_stringToFile "fi" "/etc/profile"

    touch /home/${othersChroot_userV}/.profile
    touch /etc/skel/.profile

    functionsCommon_stringToFile "alias ll=\"ls -lah --color=auto\"" "/etc/profile"
    functionsCommon_stringToFile "alias grep=\"grep --color=auto\"" "/etc/profile"
    functionsCommon_stringToFile "alias cp=\"cp -i\"" "/etc/profile"
    functionsCommon_stringToFile "alias rm=\"rm -i\"" "/etc/profile"

}

functionsChroot_setShell () {
    #setting up default shell
    case "$(cat $functionsCommon_chosenShellFileV)" in
    zsh)
        functionsCommon_log "INFO" "Setup zsh default user shell "
        functionsCommon_retryIfCommandFails "chsh $(cat $functionsCommon_chosenUserNameFileV) -s /bin/$(cat $functionsCommon_chosenShellFileV)"

        functionsCommon_stringToFile ". /usr/share/zsh/plugins/zsh-history-substring-search/zsh-history-substring-search.zsh" "/home/$othersChroot_userV/.zshrc"
        ;;

    bash)
        functionsCommon_log "INFO" "Setup bash default user shell "
        functionsCommon_retryIfCommandFails "chsh $(cat $functionsCommon_chosenUserNameFileV) -s /bin/$(cat $functionsCommon_chosenShellFileV)"
        ;;

    ksh)
        functionsCommon_log "INFO" "Setup ksh default user shell "
        functionsCommon_retryIfCommandFails "chsh $(cat $functionsCommon_chosenUserNameFileV) -s /bin/$(cat $functionsCommon_chosenShellFileV)"
        ;;

    yash)
        functionsCommon_log "INFO" "Setup yash default user shell "
        functionsCommon_retryIfCommandFails "chsh $(cat $functionsCommon_chosenUserNameFileV) -s /bin/$(cat $functionsCommon_chosenShellFileV)"
        ;;
    esac
}

functionsChroot_setLimits () {
    #setting ulimits
    functionsCommon_log "INFO" "setting ulimits"
    functionsCommon_stringToFile "\* hard nofile 100000" "/etc/security/limits.conf"
    functionsCommon_stringToFile "\* soft nofile 100000" "/etc/security/limits.conf"
    functionsCommon_stringToFile "root hard nofile 100000" "/etc/security/limits.conf"
    functionsCommon_stringToFile "root soft nofile 100000" "/etc/security/limits.conf"
    functionsCommon_stringToFile "$othersChroot_userV hard nofile 524288" "/etc/security/limits.conf"
    functionsCommon_stringToFile "$othersChroot_userV soft nofile 524288" "/etc/security/limits.conf"
}