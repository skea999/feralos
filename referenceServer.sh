#! /bin/sh

#userCheck=$(id -u)
#[ "$userCheck" != "0" ] && echo "This script must be run from root. " 0 0 && exit 1

[ "$(id -u)" != "0" ] && echo "This script must be run from root. --------------------------------------------" 0 0 && exit 1

chmod +x ./referenceServerChroot.sh
chmod +x ./referenceServerVariables.sh

. ./referenceServerVariables.sh

apt update
apt install -y mdadm btrfs-progs curl tar dosfstools

apkVersion=$(curl -sSf https://dl-cdn.alpinelinux.org/alpine/latest-stable/main/x86_64/ | sed -n 's/.*apk-tools-static-\(.*\)\.apk.*/\1/p' )

if [[ "${disk1}" == "${disk2}" ]]; then
    echo "Error: disk1 and disk2 contain the same value ('${disk1}').--------------------------------------------"
    exit 1
fi

# for i in ${disk1} ${disk2} ; do
# if [[ "${i}" == *"nvme"* ]] ;
# then
# echo $(cat /scriptv/DISK_V)p1 > /scriptv/EFI_V
# echo $(cat /scriptv/DISK_V)p2 > /scriptv/GRUB_V
# echo $(cat /scriptv/DISK_V)p3 > /scriptv/ROOT_V
# else
# echo $(cat /scriptv/DISK_V)1 > /scriptv/EFI_V
# echo $(cat /scriptv/DISK_V)2 > /scriptv/GRUB_V
# echo $(cat /scriptv/DISK_V)3 > /scriptv/ROOT_V
# fi
# done

disksMinSize=9223372036854775807

for i in ${disk1} ${disk2} ; do

diskSizeBytes="$(fdisk -l ${i} | grep -i "^Disk ${i}:" | tr ',' '\n' | grep -i 'bytes' | head -n1 | awk '{print $1}')"

diskSizeGiB=$(awk "BEGIN { printf \"%.0f\", ${diskSizeBytes}/1024/1024/1024 }")

if (( diskSizeGiB < 8 )); then
    echo "Error: Disk  is smaller than the required 8 GiB.--------------------------------------------"
    exit 1
else
    if (( diskSizeBytes < disksMinSize )); then
        echo "Disk ${i} is smaller (${diskSizeBytes}) → updating disksMinSize (${disksMinSize})--------------------------------------------"
        disksMinSize=${diskSizeBytes}
    else
        echo "The already taken disk is already the smaller (or equal) value (${disksMinSize})--------------------------------------------"
    fi
fi
done

firstPartitionSize=$((${disksMinSize}/1024/4*2-512*1024))
secondPartitionSize=$((${disksMinSize}/1024/4*1))

echo "firstPartitionSize=${firstPartitionSize}--------------------------------------------"
echo "secondPartitionSize=${secondPartitionSize}--------------------------------------------"

echo "stopping all possible partitions: --------------------------------------------"
mdadm --stop /dev/md?p*
swapoff -a

for i in ${disk1} ${disk2} ; do

echo "partitioning ${i}:--------------------------------------------"

sed -e 's/\s*\([\+0-9a-zA-Z]*\).*/\1/' << EOF  | fdisk -W always ${i}
g# create new GPT partition
n# add new partition
1# partition number
# default - first sector 
+512M# partition size
t# change partition type
1# EFI
n# add new partition
2# partition number
# default - first sector 
+${firstPartitionSize}K# partition size
t# change partition type
2# partition number
43# Linux raid
n# add new partition
3# partition number
# default - first sector 
+${secondPartitionSize}K# partition size
t# change partition type
3# partition number
19# Linux swap / Solaris
n# add new partition
4# partition number
# default - first sector 
# partition size
t# change partition type
4# partition number
20# Linux filesystem
w# write partition table and exit
EOF

echo "EFI ${i}1:--------------------------------------------"
mkfs.vfat -F 32 ${i}1
fatlabel ${i}1 EFI$(echo ${i} | cut -d'/' -f3 | tr '[:lower:]' '[:upper:]')

echo "swap ${i}3:--------------------------------------------"
mkswap -L swap-$(echo ${i} | cut -d'/' -f3) ${i}3

echo "partition ${i}4:--------------------------------------------"
mkfs.btrfs -L data-$(echo ${i} | cut -d'/' -f3) ${i}4

done


modprobe raid1

echo "making second raid:--------------------------------------------"
echo "y" | mdadm --create --level=1 --raid-devices=2 /dev/md0 ${disk1}2 ${disk2}2 --quiet --force

echo "results second raid:--------------------------------------------"
cat /proc/mdstat 

echo "partitioning /dev/md0:--------------------------------------------"
sed -e 's/\s*\([\+0-9a-zA-Z]*\).*/\1/' << EOF  | fdisk -W always /dev/md0
g# create new GPT partition
n# add new partition
1# partition number
# default - first sector 
+512M# partition size
t# change partition type
20# Linux filesystem
n# add new partition
2# partition number
# default - first sector 
# partition size
t# change partition type
2# partition number
20# Linux filesystem
w# write partition table and exit
EOF

echo "partitionig btrfs /dev/md0p1--------------------------------------------"
mkfs.btrfs -L boot /dev/md0p1

echo "partitionig btrfs /dev/md0p2--------------------------------------------"
mkfs.btrfs -L root /dev/md0p2

echo "mounting btrfs /dev/md0p2--------------------------------------------"
mount -o compress=zstd:3,autodefrag,ssd /dev/md0p2 ${chrootDir}

btrfs subvolume create ${chrootDir}/@
btrfs subvolume create ${chrootDir}/@swap
btrfs subvolume create ${chrootDir}/@home
btrfs subvolume create ${chrootDir}/@opt
btrfs subvolume create ${chrootDir}/@root
btrfs subvolume create ${chrootDir}/@srv
btrfs subvolume create ${chrootDir}/@tmp
btrfs subvolume create ${chrootDir}/@var
btrfs subvolume create ${chrootDir}/@var-lib-ex
btrfs subvolume create ${chrootDir}/@var-lib-libvirt
btrfs subvolume create ${chrootDir}/@var-lib-incus
btrfs subvolume create ${chrootDir}/@var-lib-containerd
btrfs subvolume create ${chrootDir}/@var-lib-docker
btrfs subvolume create ${chrootDir}/@var-lib-podman
btrfs subvolume create ${chrootDir}/@var-cache
btrfs subvolume create ${chrootDir}/@var-log
btrfs subvolume create ${chrootDir}/@var-opt
btrfs subvolume create ${chrootDir}/@var-spool
btrfs subvolume create ${chrootDir}/@var-tmp
btrfs subvolume create ${chrootDir}/@usr-local

echo "umount ${chrootDir}: --------------------------------------------"
umount ${chrootDir}
echo "$(mount | grep mnt)"

echo "mount: --------------------------------------------"

mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@ /dev/md0p2 ${chrootDir}


mkdir -p ${chrootDir}/swap
mount -t btrfs -o compress=no,autodefrag,ssd,nodatacow,subvol=@swap /dev/md0p2 ${chrootDir}/swap
chattr +C ${chrootDir}/swap
btrfs property set ${chrootDir}/swap compression none



mkdir -p ${chrootDir}/home
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@home /dev/md0p2 ${chrootDir}/home


mkdir -p ${chrootDir}/opt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@opt /dev/md0p2 ${chrootDir}/opt


mkdir -p ${chrootDir}/root
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@root /dev/md0p2 ${chrootDir}/root


mkdir -p ${chrootDir}/srv
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@srv /dev/md0p2 ${chrootDir}/srv
chattr +C ${chrootDir}/srv
btrfs property set ${chrootDir}/srv compression none



mkdir -p ${chrootDir}/tmp
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,noexec,nosuid,subvol=@tmp /dev/md0p2 ${chrootDir}/tmp


mkdir -p ${chrootDir}/var
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var /dev/md0p2 ${chrootDir}/var


mkdir -p ${chrootDir}/var/lib/ex
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-lib-ex /dev/md0p2 ${chrootDir}/var/lib/ex
chattr +C ${chrootDir}/var/lib/ex
btrfs property set ${chrootDir}/var/lib/ex compression none


mkdir -p ${chrootDir}/var/lib/libvirt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-libvirt /dev/md0p2 ${chrootDir}/var/lib/libvirt


mkdir -p ${chrootDir}/var/lib/incus
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-incus /dev/md0p2 ${chrootDir}/var/lib/incus


mkdir -p ${chrootDir}/var/lib/containerd
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-containerd /dev/md0p2 ${chrootDir}/var/lib/containerd


mkdir -p ${chrootDir}/var/lib/docker
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-docker /dev/md0p2 ${chrootDir}/var/lib/docker


mkdir -p ${chrootDir}/var/lib/podman
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-lib-podman /dev/md0p2 ${chrootDir}/var/lib/podman


mkdir -p ${chrootDir}/var/cache
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-cache /dev/md0p2 ${chrootDir}/var/cache


mkdir -p ${chrootDir}/var/log
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-log /dev/md0p2 ${chrootDir}/var/log
chattr +C ${chrootDir}/var/log
btrfs property set ${chrootDir}/var/log compression none



mkdir -p ${chrootDir}/var/opt
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@var-opt /dev/md0p2 ${chrootDir}/var/opt


mkdir -p ${chrootDir}/var/spool
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-spool /dev/md0p2 ${chrootDir}/var/spool
chattr +C ${chrootDir}/var/spool
btrfs property set ${chrootDir}/var/spool compression none



mkdir -p ${chrootDir}/var/tmp
mount -t btrfs -o compress=no,autodefrag,ssd,nodev,noexec,nosuid,nodatacow,subvol=@var-tmp /dev/md0p2 ${chrootDir}/var/tmp
chattr +C ${chrootDir}/var/tmp
btrfs property set ${chrootDir}/var/tmp compression none



mkdir -p ${chrootDir}/usr/local
mount -t btrfs -o compress=zstd:3,autodefrag,ssd,subvol=@usr-local /dev/md0p2 ${chrootDir}/usr/local



mkdir ${chrootDir}/boot
mount -t btrfs -o compress=no,autodefrag,ssd /dev/md0p1 ${chrootDir}/boot

mkdir ${chrootDir}/boot/efi
mount ${disk1}1 ${chrootDir}/boot/efi

mkdir ${chrootDir}/boot/efi2
mount ${disk2}1 ${chrootDir}/boot/efi2

mkdir -p ${chrootDir}/data/data-$(echo ${disk1} | cut -d'/' -f3)
mount -t btrfs -o compress=zstd:3,autodefrag,ssd ${disk1}4 ${chrootDir}/data/data-$(echo ${disk1} | cut -d'/' -f3)

mkdir -p ${chrootDir}/data/data-$(echo ${disk2} | cut -d'/' -f3)
mount -t btrfs -o compress=zstd:3,autodefrag,ssd ${disk2}4 ${chrootDir}/data/data-$(echo ${disk2} | cut -d'/' -f3)


mkdir ${chrootDir}/sys
mount --rbind /sys ${chrootDir}/sys && mount --make-rslave ${chrootDir}/sys
mkdir ${chrootDir}/dev
mount --rbind /dev ${chrootDir}/dev && mount --make-rslave ${chrootDir}/dev
mkdir ${chrootDir}/proc
mount --rbind /proc ${chrootDir}/proc && mount --make-rslave ${chrootDir}/proc

echo "mount results: --------------------------------------------"
echo "$(mount | grep mnt)"


##########forse è meglio con itarball se ci sono#########
curl -LO ${mirror}/latest-stable/main/${arch}/apk-tools-static-${apkVersion}.apk
tar -xzf apk-tools-static-*.apk
./sbin/apk.static -X ${mirror}/latest-stable/main -U --allow-untrusted -p ${chrootDir} --initdb add alpine-base

###############################################################################

mdadm --detail --scan > ${chrootDir}/etc/mdadm.conf

cp ./referenceServerChroot.sh ${chrootDir}/referenceServerChroot.sh
cp ./referenceServerVariables.sh ${chrootDir}/referenceServerVariables.sh

chmod +x ${chrootDir}/referenceServerChroot.sh
chmod +x ${chrootDir}/referenceServerVariables.sh

chroot ${chrootDir} /bin/ash /referenceServerChroot.sh


rm -rf ${chrootDir}/referenceServerChroot.sh
rm -rf ${chrootDir}/referenceServerVariables.sh

umount -R ${chrootDir}

#reboot
