#! /bin/bash
 

#are you on voidlinux? than you need to install ecc. #todo #da fare in maniera universale, che possa essere installato da qualsiasi versione di linux
#check if the scrit is runned as root
clear
echo
echo "Checking if the script is execute by root user "
echo
USERSC=$(id -u)
[ "$USERSC" != "0" ] && echo "This script must be run from root. " 0 0 && exit 1


echo "Please wait the preparation of the installer "
echo

#connect to internet #need future implementation
echo "Setting up DNS "
echo
echo "nameserver 8.8.8.8" > /etc/resolv.conf

#update xbps
echo "Update xbps "
echo
SSL_NO_VERIFY_PEER=true xbps-install -Sy xbps
echo
#install dialog required to this installer
echo "Installing dialog "
echo
xbps-install -Sy xbps dialog curl

#create dialog config
echo
echo "Creating dialog configuration "
echo
rm -rf ~/.dialogrc
dialog --create-rc ~/.dialogrc
sed -i "s/OFF/ON/g" ~/.dialogrc
#sed -i "s/CYAN/BLACK/g" ~/.dialogrc
#sed -i "s/BLUE/CYAN/g" ~/.dialogrc
#sed -i "s/GREEN/CYAN/g" ~/.dialogrc
#sed -i "s/YELLOW/BLACK/g" ~/.dialogrc

#make a directory to store used variables
echo "Making scriptv folder "
echo
mkdir /scriptv

#making appo things
echo "Making support variable RV_V "
echo
touch /scriptv/RV_V


#Functions
echo
echo "Defining functions "
echo
echo
echo "verify_f "
echo
verify_f () {
        touch /scriptv/VX_V
    	echo "1" > /scriptv/VX_V
    	RET=1
    	while [ "$(cat /scriptv/VX_V)" !=  "0" ]; do
        	$1
		RET=$?
        	echo
        	if [ $RET -eq 0 ]; then
			echo "0" > /scriptv/VX_V
		else
			echo "Retrying "
        	fi
    done
    }

echo
echo "messagebox_f "
echo

messagebox_f () {
    dialog --colors --title "\Z1\Zb$1 " --msgbox "$2 " 0 0
}
echo "inputbox_f "
echo
inputbox_f () {
    touch $1
    echo "" > $1
    APP=""

    while  [ "$(cat $1)" =  "" ] ; do
        dialog --colors --title "\Z1\Zb$2 " --inputbox "\Z1$APP\n$3" 0 0 $4 2> $1
        APP="Please try again"
    done
    }

echo "passwordbox_f "
echo
passwordbox_f () {
    touch $1
    echo "ciao" > $1
    touch ${1}2
    echo "ciao2" > ${1}2
    APP=""
    
    while [ "$(cat $1)" =  "" -o "$(cat $1)" != "$(cat ${1}2)" ]; do
        dialog --colors --insecure --passwordbox "\Z1$APP\nEnter $2 password: " 0 0 2> $1
        echo
        dialog --colors --insecure --passwordbox "\Z1$APP\nEnter $2 password for the second time: " 0 0 2> ${1}2
        echo
        APP="Please try again"
    done
    }

echo "menu_select_f "
echo
menu_select_f () {
    touch $1
    echo "" > $1
    APP=""
    AR=()

    while read N S ; do
        AR+=($N "$S")
    done < $2
    while  [ "$(cat $1)" =  "" ] ; do
        dialog --colors --menu "\Z1$APP\nSelect a $3 " 0 0 0 "${AR[@]}" 2> $1
        APP="Please try again"
    done
    echo "" > $2
    }

echo "yesno_f "
echo
yesno_f () {
    touch $1
    echo "ciao" > $1
    APP=""

    while  [ "$(cat $1)" =  "ciao" ] ; do
        dialog --colors --default-button "yes" --yesno "\Z1$APP\nDo you need $2? " 0 0
        echo $? > $1
        APP="Please try again"
    done
    #0 means yes 1 means no
    }


#ultimate check, if no than start to the begin
echo "Inizializing RV_V "
echo
echo "1" > /scriptv/RV_V

echo "While for choices "
echo
while  [ "$(cat /scriptv/RV_V)" =  "1" ] ; do

#initial message
echo
echo "Initial message "
echo
messagebox_f "Welcome to simple Void Linux install script." "This script will install Void Linux. \nWhat the script will do: \n1. Encrypted boot and root, \n2. /boot/efi in fat32 unencrypted, 512 Mb reccomanded, \n3. /boot in ext4 encrypted in LUKS1, 4 Gb reccomended, \n4. / in btrfs encrypted in LUKS2, \n5. / key in initrfs for typing password once, \n6. no LVM, instead there are btrfs sobvolumes, \n7. swapfile in a btrfs subvolume with no COW and no compression, 16 Gb, \n8. subvolumes schemes are copyed by OpenSuse configuration, in that way nothing unnecessary will be included into snapshots, \n9. fstab optimizations for ssds and compression, \n10. snapper configured and first snapshoot taked, \n11. Void Linux musl 64bit only, \n12. free software only with a lxd container for glibc and non free software, \n13. s6 init instead of runit on the main system and on the container, \n14. the best of all programs (like chrony for ntp, ecc.), \n15. KDE Plasma desktop with xorg, \n16. ClamAV and ClamTK for antivirus software, \n17. a lot of software, all you will need! \n18. flatpak on userspace to install necessary proprietary software (like firefox DRM to what Netflix or Amazon Prime Video), \nMake sure to follow the instructions, all in detailed and well documented, if you want to modify it just do it! \nMake shure that you are in the right directory (cd gab-void-linux-installer)."

#input repos
echo "Input repos "
echo
inputbox_f /scriptv/REPO_V "Repository " "Give me the ip of you machine for the repository (or type alpha.de.repo.voidlinux.org) " "alpha.de.repo.voidlinux.org"

#input hostname
echo "Input hostname "
echo
inputbox_f /scriptv/HOSTNAME_V "Hostname" "Give me the Hostname " $(cat /scriptv/HOSTNAME_V)

#input volumes password
echo "Input volumes password "
echo
passwordbox_f /scriptv/CRYPT_PSW "Decryption"

#input username
echo "Input username "
echo
inputbox_f /scriptv/USR_V "User" "Give me the Username " $(cat /scriptv/USR_V)

#input user and root password
echo "Input user and root password "
echo
passwordbox_f /scriptv/ROOT_PSW "User and Root"

#language #don't know if it works
echo "Input language "
echo
ls -d /usr/share/locale/*/ | sed "s/\/usr\/share\/locale\///g" | sed 's/.$//' > /scriptv/RV_V
menu_select_f /scriptv/LANG_V /scriptv/RV_V "Language"

#keymap
echo "Input keymaps "
echo
find /usr/share/kbd/keymaps/ -type f -iname "*.map.gz" | sed "s/.*\///; s/\.map.gz//" | sort > /scriptv/RV_V
menu_select_f /scriptv/KEYMAP_V /scriptv/RV_V "Keymap"


#timezone
echo "Input timezone "
echo
touch /scriptv/TIMEZONE3_V
echo "" > /scriptv/TIMEZONE3_V
touch /scriptv/TIMEZONE2_V
echo "" > /scriptv/TIMEZONE2_V
APP=""
while  [ "$(cat /scriptv/TIMEZONE2_V)" =  "" ] ; do
    touch /scriptv/TIMEZONE_V
    echo "" > /scriptv/TIMEZONE_V
    APP=""
    touch /scriptv/RV_V
    echo "" > /scriptv/RV_V
    #ls -d /usr/share/zoneinfo/[[:upper:]]*/ | sed "s/\/usr\/share\/zoneinfo\///g"

    ls /usr/share/zoneinfo/ | grep "^[A-Z]" > /scriptv/RV_V
    AR=()

    while read N S ; do
        AR+=($N "$S")
    done < /scriptv/RV_V

    while  [ "$(cat /scriptv/TIMEZONE_V)" =  "" ] ; do
        dialog --colors --menu "\Z1$APP\nSelect Timezone " 0 0 0 "${AR[@]}" 2> /scriptv/TIMEZONE_V
        APP="Please try again"
    done

    touch /scriptv/TIMEZONE2_V
    echo "" > /scriptv/TIMEZONE2_V
    APP=""
    touch /scriptv/RV_V
    echo "" > /scriptv/RV_V

    ls /usr/share/zoneinfo/"$(cat /scriptv/TIMEZONE_V)" | sed "s/\/usr\/share\/zoneinfo\///g" > /scriptv/RV_V
    AR=()

    while read N S ; do
        AR+=($N "$S")
    done < /scriptv/RV_V
 
    dialog --colors --menu "\Z1$APP\nSelect Timezone " 0 0 0 "${AR[@]}" 2> /scriptv/TIMEZONE2_V
    APP="Please try again"

done
echo "Setting correct timezone "
echo
if [ "$(cat /scriptv/TIMEZONE_V)" = "$(cat /scriptv/TIMEZONE2_V)" ]; 
then
    echo "/usr/share/zoneinfo/$(cat /scriptv/TIMEZONE_V)" > /scriptv/TIMEZONE3_V
else
    echo "/usr/share/zoneinfo/$(cat /scriptv/TIMEZONE_V)/$(cat /scriptv/TIMEZONE2_V)" > /scriptv/TIMEZONE3_V
fi
echo "" > /scriptv/RV_V

#video drivers
echo "Input video drivers "
echo
echo $'intel\namd\nnvidia\n" "' > /scriptv/RV_V
menu_select_f /scriptv/VIDEO_DRIVERS_V /scriptv/RV_V "Video Drivers"


#editor
echo "Input editor "
echo
echo $'nano\nvim\nemacs\nne\nneovim\ntilde' > /scriptv/RV_V
menu_select_f /scriptv/EDITOR_V /scriptv/RV_V "Editor"

#shell
echo "Input shell "
echo
echo $'bash\nfish-shell\nzsh' > /scriptv/RV_V
menu_select_f /scriptv/SHELL_V /scriptv/RV_V "Shell"



#video_drivers #OTHER VARIABLES THAT NEED A FUTURE IMPLEMENTATION



#desktop_environment
echo "Input desktop environment "
echo
echo $'kde\ngnome\nxfce\nmate\ncinnamon\nlxde\nlxqt\nenlightenment\nnone' > /scriptv/RV_V
menu_select_f /scriptv/DE_V /scriptv/RV_V "Desktop Environment "

#bluetooth
echo "Input bluethoot "
echo
yesno_f /scriptv/BT_V "Bluetooth"

#printers
echo "Input printers "
echo
yesno_f /scriptv/PRINTERS_V "Printers"

#virtualization
echo "Input virtualization "
echo
yesno_f /scriptv/VIRT_V "Virtualization (qemu, kvm, virt-manager)"

#lxd Containers
echo "Input lxd "
echo
yesno_f /scriptv/LXD_V "Containers (lxd)"

#common Software
echo "Input common software "
echo
touch /scriptv/CS_V
echo "" > /scriptv/CS_V
APP=""
touch /scriptv/RV_V
echo 1 > /scriptv/RV_V

while  [ "$(cat /scriptv/RV_V)" = 1 ] ; do
    while  [ "$(cat /scriptv/RV_V)" = 1 ] ; do
        dialog --colors --checklist "\Z1$APP\nSelect other Common Software " 0 0 0 "gedit" "gedit" on "gedit-plugins" "gedit-plugins" on "gimp" "gimp" on "inkscape" "inkscape" on "blender" "blender" on "okular" "okular" on "obs" "obs" on "libreoffice" "libreoffice" on "calligra" "calligra" off "flatpak" "flatpak" on "firefox" "firefox" on "chromium" "chromium" off "konqueror" "konqueror" on "ntfs-3g" "ntfs-3g" on "htop" "htop" on "kinfocenter" "kinfocenter" on "gparted" "gparted" on "thunderbird" "thunderbird" on "vlc" "vlc" on "ark" "ark" on "lmms" "lmms" on "kdenlive" "kdenlive" on "shotcut" "shotcut" off "knotes" "knotes" on "deluge" "deluge" on "deluge-gtk" "deluge-gtk" on "filezilla" "filezilla" off "remmina" "remmina" on "vscode" "vscode" on "bluefish" "bluefish" off "kdevelop" "kdevelop" on "sequeler" "sequeler" on "godot" "godot" off "freecad" "freecad" on  "cura" "cura" off "0ad" "0ad" off "qalculate" "qalculate" on "git" "git" off "kdeconnect" "kdeconnect" on "konsole" "konsole" on "tilix" "tilix" on "nmap" "nmap" on "zenmap" "zenmap" on 2> /scriptv/CS_V
        echo $? > /scriptv/RV_V
    done
    echo 1 > /scriptv/RV_V
    dialog --colors --default-button "yes" --yesno "\Z1$APP\nAre you shure of selected software? " 0 0
    echo $? > /scriptv/RV_V
    APP="Please try again"
done
echo "" > /scriptv/RV_V

echo "1" > /scriptv/RV_V

#format disk
echo "Input disk "
echo
while [ "$(cat /scriptv/RV_V)" = 1 ] ; do
    lsblk -dp | grep -o '^/dev[^ ]*' | grep -e $*vd. -e $*sd. -e $*nvme > /scriptv/RV_V
    menu_select_f /scriptv/DISK_V /scriptv/RV_V "Disk to install Void Linux: "

sed -e 's/\s*\([\+0-9a-zA-Z]*\).*/\1/' << EOF  | fdisk $(cat /scriptv/DISK_V)
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

    fdisk -l > /scriptv/RV_V
    dialog --colors --title "Disk configurtion " --textbox /scriptv/RV_V 0 0
    echo "" > /scriptv/RV_V
    yesno_f /scriptv/RV_V "this Disk configuration"

done
echo "" > /scriptv/RV_V

#assign variables for disks
echo "Assign variables for disks "
echo
touch /scriptv/EFI_V
touch /scriptv/GRUB_V
touch /scriptv/ROOT_V
echo "" > /scriptv/EFI_V
echo "" > /scriptv/GRUB_V
echo "" > /scriptv/ROOT_V

if [[ "$(cat /scriptv/DISK_V)" == *"nvme"* ]] ;
then
echo $(cat /scriptv/DISK_V)p1 > /scriptv/EFI_V
echo $(cat /scriptv/DISK_V)p2 > /scriptv/GRUB_V
echo $(cat /scriptv/DISK_V)p3 > /scriptv/ROOT_V
else
echo $(cat /scriptv/DISK_V)1 > /scriptv/EFI_V
echo $(cat /scriptv/DISK_V)2 > /scriptv/GRUB_V
echo $(cat /scriptv/DISK_V)3 > /scriptv/ROOT_V
fi

#ultimate check, if no than start to the begin
echo "Ultimate check "
echo
echo "1" > /scriptv/RV_V
yesno_f /scriptv/RV_V "this configuration (if no then start to the begin)"
done

echo
echo
echo "We are making things done thank you for your patience "
echo

#OTHER VARIABLES THAT NEED A FUTURE IMPLEMENTATION
#language

#crypting partitions
echo "Encrypting partitions "
echo
echo "Grub "
echo
cat /scriptv/CRYPT_PSW | cryptsetup luksFormat --type luks1 $(cat /scriptv/GRUB_V)
echo "Root "
echo
cat /scriptv/CRYPT_PSW | cryptsetup luksFormat $(cat /scriptv/ROOT_V)

#open partitions
echo "Open Encrypted partitions "
echo
echo "Grub "
echo
cat /scriptv/CRYPT_PSW | cryptsetup luksOpen $(cat /scriptv/GRUB_V) luks-$(blkid -o value -s UUID $(cat /scriptv/GRUB_V))
echo "Root "
echo
cat /scriptv/CRYPT_PSW | cryptsetup luksOpen $(cat /scriptv/ROOT_V) luks-$(blkid -o value -s UUID $(cat /scriptv/ROOT_V))

#creating file systems
echo "Creating file systems "
echo
echo "Grub "
echo
mkfs.ext4 -L grub /dev/mapper/luks-$(blkid -o value -s UUID $(cat /scriptv/GRUB_V))
echo
echo "Root "
echo
echo "Creating volume group "
vgcreate voidvm /dev/mapper/luks-$(blkid -o value -s UUID $(cat /scriptv/ROOT_V))
echo
echo "Creating logical volume of voidvm "
echo
lvcreate --name root -l 100%FREE voidvm
echo "Creating file systems for root "
echo
mkfs.btrfs -L root /dev/voidvm/root
echo "Efi "
echo
mkfs.vfat -F 32 $(cat /scriptv/EFI_V)

#mounting file systems
echo
echo "Mounting filesystems "
echo
echo "Root "
echo
mount -o compress=zstd:3,autodefrag,ssd /dev/voidvm/root /mnt

#creating sobvolumes #fare dentro un ciclo for
echo "Creating subvolumes "
echo
btrfs subvolume create /mnt/@
btrfs subvolume create /mnt/@swapfile
btrfs subvolume create /mnt/@home
btrfs subvolume create /mnt/@opt
btrfs subvolume create /mnt/@root
btrfs subvolume create /mnt/@srv
btrfs subvolume create /mnt/@tmp
btrfs subvolume create /mnt/@var
btrfs subvolume create /mnt/@var-lib-ex
btrfs subvolume create /mnt/@var-lib-flatpak
btrfs subvolume create /mnt/@var-lib-libvirt
btrfs subvolume create /mnt/@var-lib-lxd
btrfs subvolume create /mnt/@var-lib-containerd
btrfs subvolume create /mnt/@var-lib-docker
btrfs subvolume create /mnt/@var-cache
btrfs subvolume create /mnt/@var-log
btrfs subvolume create /mnt/@var-opt
btrfs subvolume create /mnt/@var-spool
btrfs subvolume create /mnt/@var-tmp
btrfs subvolume create /mnt/@usr-local


umount /mnt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@ /dev/voidvm/root /mnt


mkdir -p /mnt/swapfile
mount -t btrfs -o compress=no,autodefrag,ssd,nodatacow,subvol=@swapfile /dev/voidvm/root /mnt/swapfile
chattr +C /mnt/swapfile
btrfs property set /mnt/swapfile compression none



mkdir -p /mnt/home
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@home /dev/voidvm/root /mnt/home


mkdir -p /mnt/opt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@opt /dev/voidvm/root /mnt/opt


mkdir -p /mnt/root
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@root /dev/voidvm/root /mnt/root


mkdir -p /mnt/srv
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@srv /dev/voidvm/root /mnt/srv
chattr +C /mnt/srv
btrfs property set /mnt/srv compression none



mkdir -p /mnt/tmp
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,noexec,nosuid,subvol=@tmp /dev/voidvm/root /mnt/tmp


mkdir -p /mnt/var
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var /dev/voidvm/root /mnt/var


mkdir -p /mnt/var/lib/ex
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-lib-ex /dev/voidvm/root /mnt/var/lib/ex
chattr +C /mnt/var/lib/ex
btrfs property set /mnt/var/lib/ex compression none



mkdir -p /mnt/var/lib/flatpak
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-flatpak /dev/voidvm/root /mnt/var/lib/flatpak


mkdir -p /mnt/var/lib/libvirt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-libvirt /dev/voidvm/root /mnt/var/lib/libvirt


mkdir -p /mnt/var/lib/lxd
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-lxd /dev/voidvm/root /mnt/var/lib/lxd


mkdir -p /mnt/var/lib/containerd
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-containerd /dev/voidvm/root /mnt/var/lib/containerd


mkdir -p /mnt/var/lib/docker
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-docker /dev/voidvm/root /mnt/var/lib/docker


mkdir -p /mnt/var/cache
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-cache /dev/voidvm/root /mnt/var/cache


mkdir -p /mnt/var/log
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-log /dev/voidvm/root /mnt/var/log
chattr +C /mnt/var/log
btrfs property set /mnt/var/log compression none



mkdir -p /mnt/var/opt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-opt /dev/voidvm/root /mnt/var/opt


mkdir -p /mnt/var/spool
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-spool /dev/voidvm/root /mnt/var/spool
chattr +C /mnt/var/spool
btrfs property set /mnt/var/spool compression none



mkdir -p /mnt/var/tmp
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-tmp /dev/voidvm/root /mnt/var/tmp
chattr +C /mnt/var/tmp
btrfs property set /mnt/var/tmp compression none



mkdir -p /mnt/usr/local
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@usr-local /dev/voidvm/root /mnt/usr/local


#btrfs subvolume create /mnt/.snapshots
#mkdir -p /mnt/.snapshots
#mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=.snapshots /dev/mapper/luks-$(blkid -o value -s UUID $(cat /scriptv/ROOT_V)) /mnt/.snapshots

#btrfs subvolume create /mnt/@home/.snapshots
#mkdir -p /mnt/home/.snapshots
#mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@home/.snapshots /dev/mapper/luks-$(blkid -o value -s UUID $(cat /scriptv/ROOT_V)) /mnt/home/.snapshots

echo "Grub "
echo
mkdir /mnt/boot
mount /dev/mapper/luks-$(blkid -o value -s UUID $(cat /scriptv/GRUB_V)) /mnt/boot
echo "Efi "
echo
mkdir /mnt/boot/efi
mount $(cat /scriptv/EFI_V) /mnt/boot/efi



#prepairing for chroot #funzione, if per controllo mkdir if per controllo esecuzione exit 0 e ciclo for
echo
echo "Mounting sys "
echo
mkdir /mnt/sys
mount --rbind /sys /mnt/sys && mount --make-rslave /mnt/sys
echo "Mounting dev "
echo
mkdir /mnt/dev
mount --rbind /dev /mnt/dev && mount --make-rslave /mnt/dev
echo "Mounting proc "
echo
mkdir /mnt/proc
mount --rbind /proc /mnt/proc && mount --make-rslave /mnt/proc

#setting right repos
#to automatic install put the fingerprint in /var/db/xbps/keys
echo "Setting repos "
echo
REPO=http://$(cat /scriptv/REPO_V)/current/musl

#architecture
echo "Setting arch "
echo
ARCH=x86_64-musl

#installing the inizial setup
echo "Installing system in /mnt"
echo
XBPS_ARCH=$ARCH 
verify_f "xbps-install -Sy -r /mnt -R "$REPO" base-system cryptsetup grub-x86_64-efi lvm2 " <<EOF

y

EOF

#taking variables to chroot
echo
echo "Coping variables in /mnt "
echo
mkdir /mnt/scriptv
cp -r /scriptv /mnt/scriptv

#taking the second script to chroot
echo "Coping scripts to /mnt "
echo
cp -r ./others /mnt/scriptv/others
chmod -R +x /mnt/scriptv/others

echo "Entering chroot "
echo
chroot /mnt /bin/bash /scriptv/others/chroot.sh

echo "Ultimate message "
echo
messagebox_f "Gab Void Linux installer " "Congratulations installed success\nIf all is gone well umount /mnt with:\nsudo umount -R /mnt\nAnd than reboot\nRemember to execute other sccripts once system will boot \n\nThank you for using this script "

