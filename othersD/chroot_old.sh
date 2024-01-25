#! /bin/bash

#funzione installa pacchetto e abilita servizio parametri in input nomepacchetto, nomeservizio

echo "repository=http://$(cat /scriptv/scriptv/REPO_V)/current/musl" > /usr/share/xbps.d/00-repository-main.conf

echo "Defining functions "
echo
echo
echo "verify_f "
echo
verify_f () {
        touch /scriptv/scriptv/VX_V
    	echo "1" > /scriptv/scriptv/VX_V
    	RET=1
    	while [ "$(cat /scriptv/scriptv/VX_V)" !=  "0" ]; do
        	$1
		RET=$?
        	echo
        	if [ $RET -eq 0 ]; then
			echo "0" > /scriptv/scriptv/VX_V
		else
			echo "Retrying "
        	fi
    done
    }


USE_XORG=1
xorg_f () {
    #xorg # to take as input
    echo "Install xorg "
    echo
    verify_f "xbps-install -y xorg-minimal xhost xorg-fonts xterm"
}


#Install video drivers function

installVideo_f () {

    installPackage_f () {
        verify_f "xbps-install -Syu $1"
    }

    installCommon_f () {
        installPackage_f "mesa-dri vulkan-loader"
    }

    intel_f () {
        installPackage_f "linux-firmware-intel mesa-vulkan-intel intel-video-accel"
        declare -i CPU_VERSION=$(getIntelVersion_f)
        declare -i BROADWELL=5
        if [ $CPU_VERSION -eq $BROADWELL ]; then 
            #kernel fix
            KERNEL_CMD_FILE=/etc/dracut.conf.d/20-iommu-igfx-off.conf
            echo "Installing broadwell kernel fix in: $KERNEL_CMD_FILE"
            touch $KERNEL_CMD_FILE
            echo "intel_iommu=igfx_off" >> $KERNEL_CMD_FILE
        fi
    }

    amd_f () {
        installPackage_f "linux-firmware-amd mesa-vulkan-radeon amdvlk mesa-vdpau mesa-vaapi $( [ $USE_XORG -eq 1 ] && echo 'xf86-video-amdgpu' )" 
    }

    nvidia_f () {
        installPackage_f "$( [ $USE_XORG -eq 1 ] && echo 'xf86-video-nouveau' )"
    }

    getIntelVersion_f () {
        IN=$(cat /proc/cpuinfo | grep "model name" | awk '{print $6}' | head -n 1)
        ARR_IN=(${IN//-/ })
        CPU_SERIES=${ARR_IN[1]:0:1}
        return $CPU_SERIES
    }
    installCommon_f
    #Main
    CHOICE_VIDEO=$1


#fare un case

    if [ "$CHOICE_VIDEO" = "intel" ]; then
        echo "Intel "
        intel_f $2
    fi
    if [ "$CHOICE_VIDEO" = "amd" ]; then
        echo "Amd "
        amd_f $2
    fi
    if [ "$CHOICE_VIDEO" = "nvidia" ]; then
        echo "Nvidia "
        nvidia_f $2
    fi
}


echo
echo
echo
echo "Welcome to chroot "
echo


#setting right permissions to *
echo "Setting right permission to / "
echo
chown root:root /
chmod 755 /

#allow members of group wheel to execute any command
echo "Allow to users of the group wheel to execute any command "
echo
echo "%wheel ALL=(ALL) ALL" >> /etc/sudoers.d/99-wheel

#setting hostname 
echo "Setting hostname "
echo
echo "$(cat /scriptv/scriptv/HOSTNAME_V)" > /etc/hostname

#setting language 
echo "Setting locale "
echo 
echo "LANG=$(cat /scriptv/scriptv/LANG_V)" > /etc/locale.conf #non so se funziona perché li c'è solo it #non so neanche se funziona in musl
#echo "LANG=it_IT.UTF-8" > /etc/locale.conf #language need to take as input #questo mi sa che funziona da verificare


#setting keymap #testing
echo "Setting keymap "
echo
sed -i "s/#KEYMAP=\"es\"/KEYMAP=\"$(cat /scriptv/scriptv/KEYMAP_V)\"/g" /etc/rc.conf #language need to take as input

#if you want a glibc environment than you need to configure locales
#echo $(cat /scriptv/scriptv/LANG_V) > /etc/default/libc-locales
#echo "it_IT.UTF-8 UTF-8" >> /etc/default/libc-locales #language need to take as input
#xbps-reconfigure -f glibc-locales

#setting DNS
echo "Setting DNS "
echo
touch /etc/resolv.conf
echo "nameserver 8.8.8.8" > /etc/resolv.conf

#initial update
echo "Initial Update "
echo
echo "1"
verify_f "xbps-install -Syu"
echo "2"
SSL_NO_VERIFY_PEER=true xbps-install -Sy xbps
echo "3"
verify_f "xbps-install -Syu"

#fstab
echo
echo "Setting fstab "
echo
echo "/dev/voidvm/root     /                       btrfs    compress=zstd:3,autodefrag,ssd,subvol=@                                       0       1 " > /etc/fstab
echo "/dev/voidvm/root     /swapfile               btrfs    compress=no,autodefrag,ssd,nodatacow,subvol=@swapfile                         0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /home                   btrfs    compress=zstd:3,autodefrag,ssd,subvol=@home                                   0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /opt                    btrfs    compress=zstd:3,autodefrag,ssd,subvol=@opt                                    0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /root                   btrfs    compress=zstd:3,autodefrag,ssd,subvol=@root                                   0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /srv                    btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@srv          0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /tmp                    btrfs    compress=zstd:3,autodefrag,ssd,noexec,nosuid,subvol=@tmp                      0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var                    btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var                                    0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var/lib/ex             btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-lib-ex   0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var/lib/flatpak        btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-flatpak                        0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var/lib/libvirt        btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-libvirt                        0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var/lib/lxd            btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-lxd                            0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var/lib/containerd     btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-containerd                     0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var/lib/docker         btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-lib-docker                         0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var/cache              btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-cache                              0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var/log                btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-log      0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var/opt                btrfs    compress=zstd:3,autodefrag,ssd,subvol=@var-opt                                0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var/spool              btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-spool    0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /var/tmp                btrfs    compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-tmp      0       0 " >> /etc/fstab
echo "/dev/voidvm/root     /usr/local              btrfs    compress=zstd:3,autodefrag,ssd,subvol=@usr-local                              0       0 " >> /etc/fstab
echo "/dev/mapper/luks-$(blkid -o value -s UUID $(cat /scriptv/scriptv/GRUB_V))     /boot                   ext4     defaults                                                                      0       0 " >> /etc/fstab
echo "UUID=$(blkid -o value -s UUID $(cat /scriptv/scriptv/EFI_V))                                            /boot/efi               vfat     defaults                                                                      0       0 " >> /etc/fstab
echo "#tmpfs                                                   /tmp                    tmpfs    defaults,nosuid,nodev                                                         0       0" >> /etc/fstab
echo "/swapfile/swapfile                                       none                    swap     defaults                                                                      0       0 " >> /etc/fstab
#echo "/dev/mapper/luks-$(blkid -o value -s UUID $(cat /scriptv/scriptv/ROOT_V))      /.snapshots            btrfs    compress=zstd:3,autodefrag,ssd,subvol=.snapshots                              0       1" >> /etc/fstab
#echo "/dev/mapper/luks-$(blkid -o value -s UUID $(cat /scriptv/scriptv/ROOT_V))      /home/.snapshots       btrfs    compress=zstd:3,autodefrag,ssd,subvol=@home/.snapshots                        0       1" >> /etc/fstab

#doble check for grub installation
echo "Installing grub "
echo
verify_f "xbps-install -y grub-x86_64-efi"

#setting up grub to unlock the root partition
echo
echo "Configuring grub to unlock encrypted /boot "
echo
echo "GRUB_DISABLE_OS_PROBER=false" >> /etc/default/grub
echo "GRUB_BACKGROUND=/boot/splash.png" >> /etc/default/grub
echo "GRUB_ENABLE_CRYPTODISK=y" >> /etc/default/grub
sed -i "s/GRUB_CMDLINE_LINUX_DEFAULT=\"loglevel=4\"/GRUB_CMDLINE_LINUX_DEFAULT=\"loglevel=0 rd.lvm.vg=voidvm rd.luks.uuid=$(blkid -o value -s UUID $(cat /scriptv/scriptv/ROOT_V)) init=\/usr\/bin\/66\"/g" /etc/default/grub
cp /usr/share/void-artwork/splash.png /boot

#creating a keyfile
echo "Creating key "
echo
dd bs=1 count=64 if=/dev/urandom of=/etc/volume.key
#install cryptsetup
echo
echo "Installing cryptsetup "
echo
verify_f "xbps-install -y cryptsetup"
#add the keyfile to a slot for unlock the partition
echo
echo "Adding a key to automatically unlock partitions "
echo
echo "Grub "
echo
cat /scriptv/scriptv/CRYPT_PSW | cryptsetup luksAddKey $(cat /scriptv/scriptv/GRUB_V) /etc/volume.key
echo "Root "
echo
cat /scriptv/scriptv/CRYPT_PSW | cryptsetup luksAddKey $(cat /scriptv/scriptv/ROOT_V) /etc/volume.key

#setting right permissions to the keyfile
echo "Setting right permission to keyfile "
echo
chmod 000 /etc/volume.key
chmod -R g-rwx,o-rwx /boot
chmod -R g-rwx,o-rwx /etc/volume.key

#setting crypttab for unlock our partitions
echo "Setting crypttab "
echo
echo "luks-$(blkid -o value -s UUID $(cat /scriptv/scriptv/ROOT_V))   UUID=$(blkid -o value -s UUID $(cat /scriptv/scriptv/ROOT_V))   /etc/volume.key   luks" >> /etc/crypttab
echo "luks-$(blkid -o value -s UUID $(cat /scriptv/scriptv/GRUB_V))   UUID=$(blkid -o value -s UUID $(cat /scriptv/scriptv/GRUB_V))   /etc/volume.key   luks" >> /etc/crypttab

#double check for the kernel
echo "Install kernel "
echo
verify_f "xbps-install -y linux"

#setting keyfile in the initramfs for automatic unlocking of root partiton, this way you will enter the password once
echo
echo "Configuring dracut with key and crypttab "
echo
touch /etc/dracut.conf.d/10-crypt.conf
echo "install_items+=\" /etc/volume.key /etc/crypttab \"" >> /etc/dracut.conf.d/10-crypt.conf

#installing grub
echo "Installing grub to /boot "
echo
grub-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=void --recheck
#regenerating initramfs
#xbps-reconfigure -fa #li faccio alla fine
#generating grub configuration
#grub-mkconfig -o /boot/grub/grub.cfg #li faccio alla fine

#user 
echo
echo "Adding user "
echo
USR_V="$(cat /scriptv/scriptv/USR_V)"
useradd -m -G wheel,audio,video $USR_V # ,libvirt,kvm,lxd,socklog


#essential components
verify_f "xbps-install -Sy wget smartmontools gsmartcontrol"
#setting up a swapfile
echo "Setting swapfile "
echo
chattr +C /swapfile
echo "Disabling compression on swapfile subvolume "
echo
truncate -s 0 /swapfile/swapfile
echo "Disabling cow on swapfile subvolume "
echo
echo "Allocating swapfile "
fallocate -l 16G /swapfile/swapfile
echo
btrfs property set /swapfile compression none
echo
echo "Setting right permission to swapfile "
echo
chmod 600 /swapfile/swapfile
echo "Enable swapfile "
echo
mkswap /swapfile/swapfile

#adding 66 repos


xbps-install --repository=https://codeberg.org/mobinmob/void-66/raw/branch/master  void-unofficial-repo-66

#echo "repository=https://codeberg.org/mobinmob/void-66/raw/branch/master" > /etc/xbps.d/50-repository-unofficial-void-66.conf
#echo "repository=https://ftp.halifax.rwth-aachen.de/osdn/storage/g/a/av/avyssos/repo" >> /etc/xbps.d/50-repository-unofficial-void-66.conf
#echo "repository=https://free.nchc.org.tw/osdn//storage/g/a/av/avyssos/repo" >> /etc/xbps.d/50-repository-unofficial-void-66.conf
#echo "repository=https://mirrors.gigenet.com/OSDN//storage/g/a/av/avyssos/repo" >> /etc/xbps.d/50-repository-unofficial-void-66.conf
 
verify_f "xbps-install -Syu " <<EOF

y

EOF

verify_f "xbps-install -Syu boot-66serv void-66-services "

#installing 66
66boot-initial-setup

#wget -O /scriptv/66boot-rcdotconf https://codeberg.org/mobinmob/66-voidlinux/raw/branch/master/srcpkgs/boot-66serv/files/66boot-rcdotconf
#chmod +x /scriptv/66boot-rcdotconf
#bash /scriptv/66boot-rcdotconf
66boot-rcdotconf

#wget -O /scriptv/66boot-storage-autoconf https://codeberg.org/mobinmob/66-voidlinux/raw/branch/master/srcpkgs/boot-66serv/files/66boot-storage-autoconf
#chmod +x /scriptv/66boot-storage-autoconf
#bash /scriptv/66boot-storage-autoconf
66boot-storage-autoconf

#regenerate 66-boot
66-enable -t boot -F boot@system

#add cmd line for booting to the new init
#touch /etc/dracut.conf.d/99-cmdline.conf
#echo "kernel_cmdline=\"init=/usr/bin/66\"" >> /etc/dracut.conf.d/99-cmdline.conf


#elogind
echo "Installing elogind "
echo
verify_f "xbps-install -y elogind dbus-elogind dbus-elogind-libs dbus-elogind-x11"

#dbus
echo
echo "Enable dbus service "
echo
ln -s /etc/sv/dbus /etc/runit/runsvdir/default/
66-enable dbus
echo "Enabling elogind service "
echo
ln -s /etc/sv/elogind /etc/runit/runsvdir/default/
66-enable elogind

#cronie
echo "Installing cronie "
echo
verify_f "xbps-install -y cronie"
echo
echo "Enable service "
echo
ln -s /etc/sv/cronie /etc/runit/runsvdir/default/
66-enable cronie

#setting ntp
echo "Setting ntp (chrony) "
echo
verify_f "xbps-install -y chrony"
echo
echo "Enable chrony service "
echo
ln -s /etc/sv/chronyd /etc/runit/runsvdir/default/
66-enable chronyd
#todo add open server for ntp

#setting network manager
echo "Setting NetworkManager "
echo
verify_f "xbps-install -y NetworkManager"
echo
echo "Enabling service "
echo
ln -s /etc/sv/NetworkManager /etc/runit/runsvdir/default/
66-enable NetworkManager

#setting up timezone
echo "Setting timezone "
echo
ln -sf $(cat /scriptv/scriptv/TIMEZONE3_V) /etc/localtime 

#kernel logger klog
# sys logging syslog
echo "Setting socklog "
echo
verify_f "xbps-install -y socklog-void"
echo
echo "Enabling services "
echo
ln -s /etc/sv/socklog-unix /etc/runit/runsvdir/default/
66-enable socklog-unix
ln -s /etc/sv/klogd /etc/runit/runsvdir/default/
66-enable klogd
echo "Add user to socklog group "
echo
usermod -a -G socklog $USR_V
#ln -s /etc/sv/nanoklogd /etc/runit/runsvdir/default/

#setting samba
echo "Setting samba "
echo
verify_f "xbps-install -y samba"
echo
echo "Enabling samba service "
echo
ln -s /etc/sv/smbd /etc/runit/runsvdir/default/
66-enable smbd

#video drivers # take as input, than autodiscover it
echo
echo "Installing drivers "
echo
#old
#verify_f "xbps-install -y mesa-dri vulkan-loader"

#verify_f "xbps-install -y mesa-vulkan-radeon amdvlk xf86-video-amdgpu"

#todo add intel drivers
#todo add nvidia drivers
#todo add virtual drivers

#don't know if they are only amd, check this and grup accordingly
#verify_f "xbps-install -y mesa-vaapi"
#verify_f "xbps-install -y mesa-vdpau"

installVideo_f $(cat /scriptv/scriptv/VIDEO_DRIVERS_V)


#for snapshot
echo
echo "Install snapper "
echo
verify_f "xbps-install -y snapper"
echo
echo "Enable service "
echo
ln -s /etc/sv/snapperd /etc/runit/runsvdir/default/
#wget -O /usr/share/66/service/snapperd https://raw.githubusercontent.com/mobinmob/void-66-services/master/usr/share/66/service/snapperd
#sed -i "s/@extdepends = ( dbus )//g" /usr/share/66/service/snapperd
#66-enable snapperd #non penso che serva il servizio abilitato

#verify_f "xbps-install -y clamav clamtk" #non so se ci sono


#non essential things


#Destop Environment
case "$(cat /scriptv/scriptv/DE_V)" in

  kde)
    xorg_f
    echo
    echo "Setup kde "
    echo
    echo "Install sddm "
    echo
    verify_f "xbps-install -y sddm"
    echo
    echo "Enable service "
    echo
    ln -s /etc/sv/sddm /etc/runit/runsvdir/default/
    66-enable sddm
    echo "Install kde "
    echo
    verify_f "xbps-install -y kde5 dolphin kde5-baseapps xorg-server-xwayland spectacle "
    #export and replace /home/skikky94/.config/kdeglobals
    ;;

  gnome)
    xorg_f
    echo
    echo "Setup gnome "
    echo
    echo "Install gdm "
    echo
    verify_f "xbps-install -y mutter gdm"
    echo
    echo "Enable service "
    echo
    ln -s /etc/sv/gdm /etc/runit/runsvdir/default/
    #wget -O /usr/share/66/service/gdm https://raw.githubusercontent.com/mobinmob/void-66-services/gdm/usr/share/66/service/gdm
    #sed -i "s/@extdepends = ( dbus )//g" /usr/share/66/service/gdm
    66-enable gdm
    echo "Install gnome "
    echo
    verify_f "xbps-install -y gnome gnome-apps xorg-server-xwayland"
    ;;

  xfce)
    xorg_f
    echo
    echo "Setup xfce "
    echo
    echo "Install lightdm "
    echo
    verify_f "xbps-install -y lightdm lightdm-gtk3-greeter"
    echo
    echo "Enable service "
    echo
    ln -s /etc/sv/lightdm /etc/runit/runsvdir/default/
    66-enable lightdm
    echo "Install xfce "
    echo
    verify_f "xbps-install -y xfce4 xfce4-plugins xorg-server-xwayland"
    ;;

  mate)
    xorg_f
    echo
    echo "Setup mate "
    echo
    echo "Install gdm "
    echo
    verify_f "xbps-install -y mutter lightdm-gtk3-greeter seatd"
    echo
    echo "Enable service "
    echo
    ln -s /etc/sv/lightdm /etc/runit/runsvdir/default/
    #wget -O /usr/share/66/service/gdm https://raw.githubusercontent.com/mobinmob/void-66-services/gdm/usr/share/66/service/gdm
    #sed -i "s/@extdepends = ( dbus )//g" /usr/share/66/service/gdm
    66-enable lightdm
    echo "Install mate "
    echo
    verify_f "xbps-install -y mate mate-extra upower0 xorg-server-xwayland nm-tray mate-tweak" #consolekit
    ;;

  cinnamon)
    xorg_f
    echo
    echo "Setup cinnamon "
    echo
    echo "Install lightdm "
    echo
    verify_f "xbps-install -y lightdm lightdm-gtk3-greeter"
    echo
    echo "Enable service "
    echo
    ln -s /etc/sv/lightdm /etc/runit/runsvdir/default/
    66-enable lightdm
    echo "Install cinnamon "
    echo
    verify_f "xbps-install -y cinnamon xorg-server-xwayland"
    ;;

  lxde)
    xorg_f
    echo
    echo "Setup lxde "
    echo
    echo "Install lightdm "
    echo
    verify_f "xbps-install -y lightdm lightdm-gtk3-greeter"
    echo
    echo "Enable service "
    echo
    ln -s /etc/sv/lightdm /etc/runit/runsvdir/default/
    66-enable lightdm
    echo "Install lxde "
    echo
    verify_f "xbps-install -y lxde xorg-server-xwayland"
    ;;

  lxqt)
    xorg_f
    echo
    echo "Setup lxqt "
    echo
    echo "Install lightdm "
    echo
    verify_f "xbps-install -y lightdm lightdm-gtk3-greeter"
    echo
    echo "Enable service "
    echo
    ln -s /etc/sv/lightdm /etc/runit/runsvdir/default/
    66-enable lightdm
    echo "Install lxqt "
    echo
    verify_f "xbps-install -y lxqt xorg-server-xwayland"
    ;;

  enlightenment)
    xorg_f
    echo
    echo "Setup enlightenment "
    echo
    echo "Install gdm "
    echo
    verify_f "xbps-install -y mutter lightdm"
    echo
    echo "Enable service "
    echo
    ln -s /etc/sv/lightdm /etc/runit/runsvdir/default/
    #wget -O /usr/share/66/service/gdm https://raw.githubusercontent.com/mobinmob/void-66-services/gdm/usr/share/66/service/gdm
    #sed -i "s/@extdepends = ( dbus )//g" /usr/share/66/service/gdm
    66-enable lightdm
    echo "Install enlightenment "
    echo
    verify_f "xbps-install -y enlightenment xorg-server-xwayland"
    ;;

  none)
    echo
    echo "No Desktop environment selected "
    echo
    ;;

esac
echo
echo "Install needed things "
echo
verify_f "xbps-install -y xdg-desktop-portal xdg-desktop-portal-gtk xdg-desktop-portal-wlr xdg-user-dirs xdg-user-dirs-gtk"

#audio (and other)
echo
echo "Setup audio "
echo
verify_f "xbps-install -y pipewire"
verify_f "xbps-install -y libspa-bluetooth alsa-pipewire libjack-pipewire pulseaudio-utils gstreamer1-pipewire" #libpipewire #already instaled #pipewire-pulse #not found
#pipewire configuration 
echo
echo "Execute pipewire on login "
echo
echo "exec pipewire &" >> /etc/profile
echo "exec pipewire-pulse &" >> /etc/profile
#echo "exec pipewire-media-session &" >> /home/$(cat /scriptv/scriptv/USR_V)/.bash_profile

#for bluethoot
if [ $(cat /scriptv/scriptv/BT_V) = 0 ]; then 
    echo "Setup bluethoot "
    echo
    verify_f "xbps-install -y bluez" 
    echo
    echo "Enable service "
    echo
    ln -s /etc/sv/bluetoothd /etc/runit/runsvdir/default/
    66-enable bluetoothd
fi

#for printers
if [ $(cat /scriptv/scriptv/PRINTERS_V) = 0 ]; then 
    echo "Setup printers "
    echo
    verify_f "xbps-install -y cups"
    echo
    echo "Enabling service "
    echo
    ln -s /etc/sv/cupsd /etc/runit/runsvdir/default/
    66-enable cupsd
    echo "Install common printers drivers "
    echo
    verify_f "xbps-install -y cups-filters"
    echo
fi

#for virtualization
if [ $(cat /scriptv/scriptv/VIRT_V) = 0 ]; then 
    echo "Installing virtualizzation "
    echo
    verify_f "xbps-install -y libvirt qemu virt-manager"
    echo
    echo "Enabling services "
    echo
    ln -s /etc/sv/libvirtd /etc/runit/runsvdir/default/
    66-enable libvirtd
    ln -s /etc/sv/virtlockd /etc/runit/runsvdir/default/
    66-enable virtlockd
    ln -s /etc/sv/virtlogd /etc/runit/runsvdir/default/
    66-enable virtlogd
    echo "Adding the user to libvirt and kvm groups "
    echo
    usermod -a -G libvirt,kvm $USR_V
fi

#lxd containers
if [ $(cat /scriptv/scriptv/LXD_V) = 0 ]; then 
    echo "Installing lxd "
    echo
    verify_f "xbps-install -y lxd"
    echo
    echo "Enabling service "
    echo
    ln -s /etc/sv/lxd /etc/runit/runsvdir/default/
    66-enable lxd
    echo "Adding the user to lxd group "
    echo
    usermod -a -G lxd $USR_V
fi

#installing EDITOR
echo "Installing editor "
echo
verify_f "xbps-install -y $(cat /scriptv/scriptv/EDITOR_V)"
#setting editor as default editor
echo
echo "Setup default editor "
echo
echo "alias ll=\'ls -lah\'" >> /etc/profile

echo
echo "Setup default editor "
echo
echo "export EDITOR=$(cat /scriptv/scriptv/EDITOR_V)" >> /etc/profile

#setting up default shell
echo "Setup default shell "
echo
verify_f "xbps-install -Syu $(cat /scriptv/scriptv/SHELL_V)"

if [ "$(cat /scriptv/scriptv/SHELL_V)" = "fish-shell" ]; then
  echo
  echo "Setup default user shell "
  echo
  chsh $(cat /scriptv/scriptv/USR_V) -s /bin/fish
  echo
  echo "Setup default root shell "
  echo
  chsh root -s /bin/fish
  echo
else
  echo
  echo "Setup default user shell "
  echo
  chsh $(cat /scriptv/scriptv/USR_V) -s /bin/$(cat /scriptv/scriptv/SHELL_V)
  echo
  echo "Setup default root shell "
  echo
  chsh root -s /bin/$(cat /scriptv/scriptv/SHELL_V)
  echo
fi

#removing dash
echo "Removing dash "
echo
touch /etc/xbps.d/ignore-dash.conf
echo "ignorepkg=dash" >> /etc/xbps.d/ignore-dash.conf
xbps-remove -y dash

#if you want a glibc install you can uncomment the istruction below
#verify_f "xbps-install -y void-repo-multilib"

#other common software
echo
echo "Installing other software "
echo
verify_f "xbps-install -y bash-completion $(cat /scriptv/scriptv/CS_V)"

#setting ulimits
echo "* hard nofile 100000" >> /etc/security/limits.conf
echo "* soft nofile 100000" >> /etc/security/limits.conf
echo "root hard nofile 100000" >> /etc/security/limits.conf
echo "root soft nofile 100000" >> /etc/security/limits.conf
echo "$USR_V hard nofile 524288" >> /etc/security/limits.conf
echo "$USR_V soft nofile 524288" >> /etc/security/limits.conf



chmod -R +x /scriptv/others


#ROOT_PSW="$()"
ROOT_PSW="$(cat /scriptv/scriptv/ROOT_PSW)"

#setting root password
echo
echo "Setting root password "
echo
passwd root << EOF
$ROOT_PSW
$ROOT_PSW
EOF
echo
echo "Setting user password "
echo
passwd $USR_V << EOF
$ROOT_PSW
$ROOT_PSW
EOF

#mv /usr/bin/shutdown /usr/bin/shutdown.bak
#ln -s /etc/66/shutdown /usr/bin/
66-enable switch-initutils

#reconfigure all
echo
echo "Reconfigure all "
echo
xbps-reconfigure -fa
#regenerate grub configuration
echo
echo "regenerate grub config "
echo
grub-mkconfig -o /boot/grub/grub.cfg


echo
echo "Exiting chroot "
echo

exit
