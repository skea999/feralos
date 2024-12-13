#!/bin/sh

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
        local functionsStartUi_choses_desktopEnvironmentsV=$'gnome\nkde\nxfce\nmate\ncinnamon\nlxde\nlxqt\nenlightenment\nnone'
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
