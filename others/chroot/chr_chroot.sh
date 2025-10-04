#! /bin/bash

#funzione installa pacchetto e abilita servizio parametri in input nomepacchetto, nomeservizio

source /scriptFilesD/functionsD/functionsCommonF.sh
#not used
source /scriptFilesD/functionsD/functionsChrootD/functionsChrootHardwareDetenctionF.sh
source /scriptFilesD/othersD/othersDistributionSpecificThingsF.sh
source /scriptFilesD/functionsD/functionsChrootD/functionsChrootF.sh

echo
echo
echo
functionsCommon_log "INFO" "Welcome to chroot "
echo

functionsCommon_log "INFO" "Setting permission to scripts folder "
functionsCommon_retryIfCommandFails "chmod -R +x /scriptFilesD"

#check packaes and determine what to installl
functionsChroot_checkPackages

#resolv.conf
functionsCommon_retryIfCommandFails "echo > \"/etc/resolv.conf\""
functionsCommon_stringToFile "nameserver 208.67.222.222" "/etc/resolv.conf"
functionsCommon_stringToFile "nameserver 208.67.220.220" "/etc/resolv.conf"
#functionsCommon_stringToFile "nameserver=208.67.222.222" "/etc/resolvconf.conf"
#functionsCommon_stringToFile "nameserver=208.67.220.220" "/etc/resolvconf.conf"

#setting right repo
othersDistributionSpecificThings_setRepos

#initial update
functionsCommon_log "INFO" "Initial Update "
echo "1"
othersDistributionSpecificThings_installPackage
echo "2"
othersDistributionSpecificThings_updatePackageManager
echo "3"
othersDistributionSpecificThings_installPackage

othersDistributionSpecificThings_installPackage "$othersDistributionSpecificThings_packagesToInstallV "

#allow members of group wheel to execute any command
functionsChroot_rootPermission

#setting hostname 
functionsChroot_setHostname

#setting language #non so se funziona perché li c'è solo it
functionsChroot_setLanguage

#setting keymap 
functionsChroot_setKeymap
#if you want a glibc environment than you need to configure locales
#echo $(cat $functionsCommon_chosenKeymapFileV) > /etc/default/libc-locales
#xbps-reconfigure -f glibc-locales

#setting DNS
functionsChroot_setDns
#change the dns to open dns instead of goole dns

#fstab
functionsChroot_setFstab

#creating a keyfile and set the right permissions
functionsChroot_createKeyFile

#add the keyfile to a slot for unlock the partition
functionsChroot_addKey "$functionsCommon_chosenEncryptPasswordFileV" "$functionsCommon_grubFileV" "grub"
functionsChroot_addKey "$functionsCommon_chosenEncryptPasswordFileV" "$functionsCommon_rootFileV" "root"

#doble check for grub installation
functionsChroot_setGrub

#user 
functionsCommon_log "INFO" "Adding user "
othersChroot_userV="$(cat $functionsCommon_chosenUserNameFileV)"
functionsCommon_retryIfCommandFails "useradd -m -G wheel,audio,video $othersChroot_userV" # ,libvirt,kvm,lxd,socklog

#setting up a swapfile
functionsChroot_setSwapFile

#setting up timezone
functionsCommon_log "INFO" "Setting timezone "
functionsCommon_retryIfCommandFails "ln -sf $(cat $functionsCommon_chosenTimeZoneFileV) /etc/localtime "

#video drivers 
#functionsCommon_log "INFO" "Installing drivers "
#old

#packagesToInstallV="${packagesToInstallV[@]} mesa-vulkan-radeon amdvlk xf86-video-amdgpu"

#don't know if they are only amd, check this and grup accordingly
#packagesToInstallV="${packagesToInstallV[@]} mesa-vaapi"
#packagesToInstallV="${packagesToInstallV[@]} mesa-vdpau"


#profile
functionsChroot_setProfile 

#pipewire configuration 
#se non è in esecuzione allora esegui: if [ ! ps -aux | grep "pipewire" = *"pipewire"* ] toliere il grep come risultato 
othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_packagesPipewireV" "ln -s /usr/share/applications/pipewire.desktop /etc/xdg/autostart/pipewire.desktop"
othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_packagesPipewireV" "mkdir -p /etc/pipewire/pipewire.conf.d"
othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_packagesPipewireV" "ln -s /usr/share/examples/wireplumber/10-wireplumber.conf /etc/pipewire/pipewire.conf.d/"
othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_packagesPipewireV" "ln -s /usr/share/examples/pipewire/20-pipewire-pulse.conf /etc/pipewire/pipewire.conf.d/"

#othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_packagesPipewireV" "echo \"exec pipewire &\" >> touch /etc/skel/.profile"
#othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_packagesPipewireV" "echo \"exec pipewire-pulse &\" >> touch /etc/skel/.profile"

#installing EDITOR
functionsCommon_log "INFO" "Setup default editor "
functionsCommon_stringToFile "export EDITOR=$(cat $functionsCommon_chosenEditorFileV)" "/etc/profile"

#setting up default shell
functionsChroot_setShell

#if you want a glibc install install void-repo-multilib

#setting ulimits
functionsChroot_setLimits

othersChroot_rootPswdV="$(cat $functionsCommon_chosenUserPasswordFileV)"
#setting root password
functionsCommon_log "INFO" "Setting root password "
passwd root << EOF
$othersChroot_rootPswdV
$othersChroot_rootPswdV
EOF

#setting user password
functionsCommon_log "INFO" "Setting user password "
passwd $othersChroot_userV << EOF
$othersChroot_rootPswdV
$othersChroot_rootPswdV
EOF


#installing 66 #how to make it a general function for all systems? with a check
othersDistributionSpecificThings_install66

#dbus
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesDbusV" "dbus"

#elogind
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesElogindV" "elogind"

#cronie
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesCronieV" "cronie"

#setting ntp
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesChronyV" "chronyd"
#todo add open server for ntp

#setting network manager
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesNetworkManagerV" "NetworkManager"

#setting clamav
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesClamavV" "clamav"

#setting samba
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesSambaV" "smbd"

#for printers
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesCupsV" "cupsd"

#kernel logger klog
# sys logging syslog
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesSocklogV" "socklog-unix"

#setting klogs
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesSocklogV" "klogd"

othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_packagesSocklogV" "usermod -a -G socklog $othersChroot_userV"
#ln -s /etc/sv/nanoklogd /etc/runit/runsvdir/default/

#for bluethoot
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesBluezV" "bluetoothd"

othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_packagesSocklogV" "usermod -a -G bluetooth $othersChroot_userV"

#lxd containers
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesLxdV" "lxd"

othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_packagesLxdV" "usermod -a -G lxd $othersChroot_userV"

#for virtualization
othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesLibvirtV" "libvirtd"

othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesLibvirtV" "virtlockd"

othersDistributionSpecificThings_enableService "$othersDistributionSpecificThings_packagesLibvirtV" "virtlogd"

othersDistributionSpecificThings_checkIfInstalled "$othersDistributionSpecificThings_packagesLibvirtV" "usermod -a -G libvirt,kvm $othersChroot_userV"


#Destop Environment
functionsChroot_setDesktopEnvironment


#reconfigure all
othersDistributionSpecificThings_reconfigureAll

#regenerate grub configuration
functionsCommon_log "INFO" "regenerate grub config "
functionsCommon_retryIfCommandFails "grub-mkconfig -o /boot/grub/grub.cfg"

echo
functionsCommon_log "INFO" "Exiting chroot "
echo

exit
