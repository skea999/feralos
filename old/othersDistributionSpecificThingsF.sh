#!/bin/bash

source /scriptFilesD/functionsD/functionsCommonF.sh

##void linux
#case
#if [$(cat /etc/*release*) | grep void = *"void"* ]; than

#initial dependdencies for install the system
othersDistributionSpecificThings_initialDependenciesV="dialog curl cryptsetup btrfs-progs bash xbps bc "


#those are relative to mirrorbenchmark
othersDistributionSpecificThings_urlV="https://docs.voidlinux.org/xbps/repositories/mirrors/index.html"
othersDistributionSpecificThings_TargetPackageV="htop"
othersDistributionSpecificThings_listingUrlV="https://alpha.de.repo.voidlinux.org/current/musl/"

#it will be put at the end of the mirror #but i don't know if it is needed without void linux
othersDistributionSpecificThings_subPathV="/current/musl"


#basic packages
#base-system
#othersDistributionSpecificThings_basicPackagesV="base-system dialog opendoas cryptsetup grub-x86_64-efi lvm2 ${othersDistributionSpecificThings_packagesCronieV} ${othersDistributionSpecificThings_packagesChronyV} ${othersDistributionSpecificThings_packagesNetworkManagerV} ${othersDistributionSpecificThings_packagesSocklogV} "
othersDistributionSpecificThings_basicPackagesV="xbps-triggers base-files musl ncurses-base ncurses-libs ncurses acl attr gmp libcap coreutils findutils diffutils libgcc bash libpcre2 grep gzip zlib libmagic file sed libreadline8 gawk less eudev-libudev util-linux-common libblkid libcap-ng libuuid libfdisk libmount libsmartcols pam-libs util-linux which tar man-pages mdocml pam-base pam shadow e2fsprogs-libs e2fsprogs libzstd lzo btrfs-progs libstdc++ inih libedit liburcu dosfstools procps-ng tzdata hwids liblzma libkmod pciutils libusb usbutils iana-etc libcrypto1.1 dnssec-anchors libldns openssh dhcpcd kbd libnfnetlink libnl3 libpcap libmnl libnftnl libnetfilter_conntrack iptables libdb iproute2 libcap-progs iputils dbus-libs libssl1.1 openssl run-parts ca-certificates bzip2 liblz4 libarchive libxbps xbps nvi void-artwork traceroute ethtool kmod acpid eudev runit runit-void removed-packages linux-firmware-network cpio libaio device-mapper kpartx musl-fts dracut linux-base linux base-system dialog opendoas cryptsetup grub-x86_64-efi lvm2 ${othersDistributionSpecificThings_packagesCronieV} ${othersDistributionSpecificThings_packagesChronyV} ${othersDistributionSpecificThings_packagesNetworkManagerV} ${othersDistributionSpecificThings_packagesSocklogV} "
#cercare di includere tutti i firmware necessari
#excluded packages="linux6.1 linux-firmware-broadcom linux-firmware-nvidia linux-firmware-intel linux-firmware-amd wifi-firmware zd1211-firmware ipw2200-firmware ipw2100-firmware sudo wpa_supplicant iw f2fs-tools xfsprogs dash "

#video packages
othersDistributionSpecificThings_packagesVideoDriversfunctionsCommonV="mesa-dri vulkan-loader "
othersDistributionSpecificThings_packagesIntelV="${othersDistributionSpecificThings_packagesVideoDriversfunctionsCommonV} "
othersDistributionSpecificThings_packagesAmdV="${othersDistributionSpecificThings_packagesVideoDriversfunctionsCommonV} mesa-vulkan-radeon amdvlk amdgpu mesa-vaapi mesa-vdpau linux-firmware-amd "
#xorg amd drivers are: xf86-video-amdgpu
othersDistributionSpecificThings_packagesNvidiaV="${othersDistributionSpecificThings_packagesVideoDriversfunctionsCommonV} "

#editor packages
othersDistributionSpecificThings_packagesNanoV="nano"
othersDistributionSpecificThings_packagesVimV="vim"
othersDistributionSpecificThings_packagesNeV="ne"
othersDistributionSpecificThings_packagesNeovimV="neovim"
othersDistributionSpecificThings_packagesTildeV="tilde"

#shell packages
othersDistributionSpecificThings_packagesBashV="bash bash-completion "
othersDistributionSpecificThings_packagesZshV="zsh zsh-completions zsh-autosuggestions zsh-history-substring-search "
othersDistributionSpecificThings_packagesKshV="ksh"
othersDistributionSpecificThings_packagesYashV="yash"

#xorg minimal packages
othersDistributionSpecificThings_packagesXorgV="xorg-minimal xhost xorg-fonts dbus-elogind-x11 "

#dbus packages
othersDistributionSpecificThings_packagesDbusV="dbus"

othersDistributionSpecificThings_packagesPipewireV="pipewire"
#pipewire dependencies
othersDistributionSpecificThings_packagesPipewireEccV="${othersDistributionSpecificThings_packagesPipewireV} libspa-bluetooth alsa-pipewire libjack-pipewire pulseaudio-utils gstreamer1-pipewire "

othersDistributionSpecificThings_packagesCommonVideoV="xdg-desktop-portal xdg-desktop-portal-gtk xdg-desktop-portal-wlr xdg-user-dirs xdg-user-dirs-gtk ${othersDistributionSpecificThings_packagesElogindEccV} ${othersDistributionSpecificThings_packagesDbusV} ${othersDistributionSpecificThings_packagesPipewireEccV} "
othersDistributionSpecificThings_packagesElogindV="elogind"
#elogind dependencies
othersDistributionSpecificThings_packagesElogindEccV="${othersDistributionSpecificThings_packagesElogindV} dbus-elogind dbus-elogind-libs "

#desktop environments
othersDistributionSpecificThings_packagesSddmV="sddm"
othersDistributionSpecificThings_packagesGdmV="gdm"
othersDistributionSpecificThings_packagesLightdmV="lightdm"
othersDistributionSpecificThings_packagesKdeV="${othersDistributionSpecificThings_packagesElogindEccV} ${othersDistributionSpecificThings_packagesXorgV} ${othersDistributionSpecificThings_packagesSddmV} ${othersDistributionSpecificThings_packagesCommonVideoV} kde5 dolphin kde5-baseapps xorg-server-xwayland spectacle " #make kde lighter
othersDistributionSpecificThings_packagesGnomeV="${othersDistributionSpecificThings_packagesElogindEccV} ${othersDistributionSpecificThings_packagesXorgV} ${othersDistributionSpecificThings_packagesGdmV} ${othersDistributionSpecificThings_packagesCommonVideoV} mutter gnome gnome-apps xorg-server-xwayland " #make gnome lighter
othersDistributionSpecificThings_packagesXfceV="${othersDistributionSpecificThings_packagesElogindEccV} ${othersDistributionSpecificThings_packagesXorgV} ${othersDistributionSpecificThings_packagesLightdmV} ${othersDistributionSpecificThings_packagesCommonVideoV} xfce4 xfce4-plugins lightdm-gtk3-greeter " #make xfce lighter
#othersDistributionSpecificThings_packagesMateV="${othersDistributionSpecificThings_packagesElogindEccV} ${othersDistributionSpecificThings_packagesXorgV} ${othersDistributionSpecificThings_packagesLightdmV} ${othersDistributionSpecificThings_packagesCommonVideoV} mate mate-icon-theme-faenza mate-tweak mozo mate-extra upower0 nm-tray lightdm-gtk3-greeter "
othersDistributionSpecificThings_packagesMateV="${othersDistributionSpecificThings_packagesElogindEccV} ${othersDistributionSpecificThings_packagesXorgV} ${othersDistributionSpecificThings_packagesLightdmV} ${othersDistributionSpecificThings_packagesCommonVideoV} mate mate-icon-theme-faenza mate-tweak mozo mate-applets mate-indicator-applet mate-media mate-power-manager mate-sensors-applet mate-screensaver mate-utils caja-extensions upower0 network-manager-applet lightdm-gtk3-greeter "
othersDistributionSpecificThings_packagesCinnamonV="${othersDistributionSpecificThings_packagesElogindEccV} ${othersDistributionSpecificThings_packagesXorgV} ${othersDistributionSpecificThings_packagesLightdmV} ${othersDistributionSpecificThings_packagesCommonVideoV} cinnamon lightdm-gtk3-greeter "

#cinnamon:
# cinnamon-desktop               install   -                 5.4.2_1                254KB 
# cinnamon-menus                 install   -                 5.4.0_1                57KB 
# libgnomekbd                    install   -                 3.28.1_1               144KB 
# cinnamon-settings-daemon       install   -                 5.4.5_1                398KB 
# cinnamon-translations          install   -                 5.4.2_1                4503KB 
# rest                           install   -                 0.9.1_1                51KB 
# gnome-online-accounts          install   -                 3.46.0_1               476KB 
# libcinnamon-control-center     install   -                 5.4.7_2                13KB 
# cinnamon-control-center        install   -                 5.4.7_2                450KB 
# glxinfo                        install   -                 8.4.0_3                21KB 
# xdpyinfo                       install   -                 1.3.3_1                17KB 
# xprop                          install   -                 1.2.6_1                27KB 
# xrandr                         install   -                 1.5.2_1                44KB 
# inxi                           install   -                 3.3.24_1               499KB 
# xapps                          install   -                 2.2.14_2               174KB 
# python3-xapp                   install   -                 2.2.2_2                11KB 
# xdotool                        install   -                 3.20211022.1_1         66KB 
# cinnamon-screensaver           install   -                 5.4.2_2                175KB 
# cinnamon-session               install   -                 5.4.0_1                130KB 
# cjs                            install   -                 5.4.1_1                389KB 
# webp-pixbuf-loader             install   -                 0.0.6_1                12KB 
# gnome-backgrounds              install   -                 43_1                   31MB 
# cantarell-fonts                install   -                 0.303.1_1              119KB 
# gnome-themes-standard-metacity install   -                 3.14.2.3_2             6944B 
# gnome-themes-extra             install   -                 3.28_3                 2545KB 
# gnome-themes-standard          install   -                 3.28_3                 573B 
# gsound                         install   -                 1.0.3_1                14KB 
# libcaribou                     install   -                 0.4.21_7               67KB 
# libtimezonemap                 install   -                 0.4.6_1                2009KB 
# muffin                         install   -                 5.4.5_2                1881KB 
# libnemo                        install   -                 5.6.3_1                41KB 
# nemo                           install   -                 5.6.3_1                1110KB 
# polkit-gnome                   install   -                 0.105_8                68KB 
# python3-inotify                install   -                 0.9.6_8                25KB 
# python3-pam                    install   -                 1.8.4_7                5812B 
# python3-ptyprocess             install   -                 0.7.0_3                13KB 
# python3-pexpect                install   -                 4.8.0_3                47KB 
# python3-pytz                   install   -                 2022.7.1_1             28KB 
# python3-tinycss                install   -                 0.4_5                  35KB 
# cinnamon                       install   -                 5.4.9_2                1851KB 


othersDistributionSpecificThings_packagesLxdeV="${othersDistributionSpecificThings_packagesElogindEccV} ${othersDistributionSpecificThings_packagesXorgV} ${othersDistributionSpecificThings_packagesLightdmV} ${othersDistributionSpecificThings_packagesCommonVideoV} lxde lightdm-gtk3-greeter "
othersDistributionSpecificThings_packagesLxqtV="${othersDistributionSpecificThings_packagesElogindEccV} ${othersDistributionSpecificThings_packagesXorgV} ${othersDistributionSpecificThings_packagesLightdmV} ${othersDistributionSpecificThings_packagesCommonVideoV} lxqt lightdm-gtk3-greeter "
othersDistributionSpecificThings_packagesEnlightenmentV="${othersDistributionSpecificThings_packagesElogindEccV} ${othersDistributionSpecificThings_packagesXorgV} ${othersDistributionSpecificThings_packagesLightdmV} ${othersDistributionSpecificThings_packagesCommonVideoV} enlightenment lightdm-gtk3-greeter "
othersDistributionSpecificThings_packagesBudgieV="${othersDistributionSpecificThings_packagesElogindEccV} ${othersDistributionSpecificThings_packagesXorgV} ${othersDistributionSpecificThings_packagesLightdmV} ${othersDistributionSpecificThings_packagesCommonVideoV} budgie-desktop papirus-icon-theme"


#66 packages #but i don't know if it is needed without void linux
#othersDistributionSpecificThings_packages66V="boot-66serv void-66-services s6-rc "
othersDistributionSpecificThings_packages66V="boot-66serv void-66-services 66-void base-system-66 "

#other packages needded for services
othersDistributionSpecificThings_packagesCronieV="cronie" #added to base install packages
othersDistributionSpecificThings_packagesChronyV="chrony" #added to base install packages
othersDistributionSpecificThings_packagesNetworkManagerV="NetworkManager" #added to base install packages
othersDistributionSpecificThings_packagesSocklogV="socklog-void" #added to base install packages
othersDistributionSpecificThings_packagesSambaV="samba"
othersDistributionSpecificThings_packagesSnapperV="snapper"

othersDistributionSpecificThings_packagesClamavV="clamav"

othersDistributionSpecificThings_packagesBluezV="bluez"

othersDistributionSpecificThings_packagesCupsV="cups"
#cups dependencies
othersDistributionSpecificThings_packagesCupsEccV="${othersDistributionSpecificThings_packagesCupsV} cups-filters "

#kvm
othersDistributionSpecificThings_packagesLibvirtV="libvirt"
#kvm ddependencies
othersDistributionSpecificThings_packagesKvmV="${othersDistributionSpecificThings_packagesLibvirtV} qemu "

othersDistributionSpecificThings_packagesLxdV="lxd"

#other packages, need to be modified in othersD/othersSoftwareToInstallF
othersDistributionSpecificThings_packagesOtherV="$(cat $functionsCommon_chosenOtherSoftwareFileV)"

othersDistributionSpecificThings_packagesToInstallV="${othersDistributionSpecificThings_basicPackagesV} ${othersDistributionSpecificThings_packagesOtherV} ${othersDistributionSpecificThings_packages66V} "


othersDistributionSpecificThings_installPackage () {
    #depends on functionsCommon_retryIfCommandFails

    local othersDistributionSpecificThings_installPackage_packagesV=$1


    functionsCommon_retryIfCommandFails "xbps-install -Syu ${othersDistributionSpecificThings_installPackage_packagesV} "
}

othersDistributionSpecificThings_setRepos () {
    functionsCommon_log "INFO" "Setting the right repos"
    echo > "/usr/share/xbps.d/00-repository-main.conf"
    functionsCommon_retryIfCommandFails "echo \"repository=http://$(cat ${functionsCommon_chosenRepositoryFileV})${othersDistributionSpecificThings_subPathV}\" > /usr/share/xbps.d/00-repository-main.conf"

    #adding 66 repos
    functionsCommon_log "INFO" "adding 66 repos"
    othersDistributionSpecificThings_installPackage "--repository=https://codeberg.org/mobinmob/void-66/raw/branch/master  void-unofficial-repo-66" <<EOF

y

EOF
}

othersDistributionSpecificThings_updatePackageManager () {
    #update xbps
    SSL_NO_VERIFY_PEER=true
    othersDistributionSpecificThings_installPackage "xbps "
}

othersDistributionSpecificThings_baseSystem () {
    #depends on functionsCommon_log functionsCommon_retryIfCommandFails
    functionsCommon_log "INFO" "Installing base system "

    #carefull, it is a distribution specific thing
    XBPS_ARCH=x86_64-musl
    #install only base-system the others will be installed in chroot added to a variable and installed all togheter 
    functionsCommon_retryIfCommandFails "xbps-install -Sy -r /mnt -R \"http://$(cat ${functionsCommon_chosenRepositoryFileV})${othersDistributionSpecificThings_subPathV}\" ${othersDistributionSpecificThings_basicPackagesV} " <<EOF

y

EOF
    #

    functionsCommon_log "WARN" "$(ls -lah /mnt)"
}

#to delete because it is not used anymore
othersDistributionSpecificThings_install66 () {
    #installing 66
    functionsCommon_retryIfCommandFails '66boot-initial-setup'

    functionsCommon_retryIfCommandFails '66boot-rcdotconf'

    functionsCommon_retryIfCommandFails '66boot-storage-autoconf'

    #regenerate 66-boot
    functionsCommon_retryIfCommandFails '66-enable -v4 -t boot -F boot@system '

    #add cmd line for booting to the new init
    #echo > /etc/dracut.conf.d/99-cmdline.conf
    #functionsCommon_stringToFile "kernel_cmdline=\"init=/usr/bin/66\"" "/etc/dracut.conf.d/99-cmdline.conf"

    functionsCommon_log "INFO" "enable other necessary services "
    functionsCommon_retryIfCommandFails "66-enable switch-initutils"

}

othersDistributionSpecificThings_checkIfInstalled () {
    local othersDistributionSpecificThings_checkIfInstalled_packageV=$1
    local othersDistributionSpecificThings_checkIfInstalled_executeV=$2

    local othersDistributionSpecificThings_checkIfInstalled_packageCleanedV
    othersDistributionSpecificThings_checkIfInstalled_packageCleanedV=$(printf '%s\n' "$othersDistributionSpecificThings_checkIfInstalled_packageV" | sed -e 's/ //g')

    if [[ "$(xbps-query -l | awk '{ print $2 }' | xargs -n1 xbps-uhelper getpkgname | grep $othersDistributionSpecificThings_checkIfInstalled_packageCleanedV)" == *"$othersDistributionSpecificThings_checkIfInstalled_packageCleanedV"* ]] ;
    then
        functionsCommon_log "INFO" "$othersDistributionSpecificThings_checkIfInstalled_packageCleanedV is installed"
        functionsCommon_retryIfCommandFails "$othersDistributionSpecificThings_checkIfInstalled_executeV"
    else
        functionsCommon_log "WARN" "$othersDistributionSpecificThings_checkIfInstalled_packageCleanedV is not installed"
    fi
}

othersDistributionSpecificThings_enableService () {
    local othersDistributionSpecificThings_enableService_packageV=$1
    local othersDistributionSpecificThings_enableService_serviceV=$2

    functionsCommon_log "INFO" "Enable $othersDistributionSpecificThings_enableService_serviceV service "

    othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_enableService_packageV" "ln -s /etc/sv/${othersDistributionSpecificThings_enableService_serviceV} /etc/runit/runsvdir/default/"
    functionsCommon_log "WARN" "$(ls -lah /etc/runit/runsvdir/default/ | grep ${othersDistributionSpecificThings_enableService_serviceV})"
    othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_enableService_packageV" "66-enable ${othersDistributionSpecificThings_enableService_serviceV}"
    functionsCommon_log "WARN" "$(66-inservice ${othersDistributionSpecificThings_enableService_serviceV})"
}

othersDistributionSpecificThings_reconfigureAll () {
    functionsCommon_log "INFO" "Reconfigure all "
    functionsCommon_retryIfCommandFails "xbps-reconfigure -fa"
}