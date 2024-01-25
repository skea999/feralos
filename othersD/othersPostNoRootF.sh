#! /bin/bash
 
clear

echo 
echo "****************************************************************************************************************************"
echo "Make sure that you execute this script as regular user, you will be asked for the password to make things as root"
echo "****************************************************************************************************************************"


#adding flathpack #if you choosed to install it
#flatpak remote-add --if-not-exists flathub --user https://flathub.org/repo/flathub.flatpakrepo #da fare una volta avviato il sistema
#flatpak install flathub --user org.telegram.desktop com.microsoft.Teams com.skype.Client com.discordapp.Discord com.bitwarden.desktop com.anydesk.Anydesk


lxd init

echo "root:$UID:1" | sudo tee -a /etc/subuid /etc/subgid

lxc profile create guiapps

cat /scriptv/others/guiapps.profile | lxc profile edit guiapps

lxc profile show steam

lxc launch images:mint/una/amd64 games --profile default --profile guiapps


ROOT_PSW="$(cat /scriptv/scriptv/ROOT_PSW)"

lxc exec games passwd ubuntu <<EOF
$ROOT_PSW
$ROOT_PSW
EOF

lxc alias add ubuntu 'exec @ARGS@ --mode interactive -- /bin/sh -xac $@ubuntu - exec /bin/login -p -f '


lxc ubuntu games <<EOF

sudo su -
dpkg --add-architecture i386

apt update
apt upgrade

add-apt-repository -y ppa:kisak/kisak-mesa

add-apt-repository -y ppa:lutris-team/lutris

apt update
apt upgrade -y
#
apt install -y lutris x11-apps mesa-utils libgl1-mesa-glx libcanberra-gtk-module pulseaudio dbus-x11 wine64 wine32 libasound2-plugins:i386 libsdl2-2.0-0:i386 libdbus-1-3:i386 libsqlite3-0:i386 libgnutls30:i386 libgpg-error0:i386 libsqlite3-0:i386 libgl1-mesa-dri:i386 mesa-vulkan-drivers mesa-vulkan-drivers:i386 xserver-xorg-video-amdgpu gstreamer1.0-libav gstreamer1.0-plugins-bad 

sed -i "s/; enable-shm = yes/enable-shm = no/g" /etc/pulse/client.conf

echo export PULSE_SERVER=unix:/tmp/.pulse-native | tee --append /home/ubuntu/.profile


EOF

#lxc exec games apt install -y x11-apps mesa-utils alsa-utils

lxc config set games limits.kernel.nofile 524288

lxc restart games


echo "xhost +local:$(cat /scriptv/scriptv/USR_V)" >> /home/$(cat /scriptv/scriptv/USR_V)/.bash_profile





#this does not work, need sed applied on root and home
#seting up snapper for snapshoots
#echo "TIMELINE_MIN_AGE=\"1800\"" > /usr/share/snapper/config-templates/default
#echo "TIMELINE_LIMIT_HOURLY=\"1\"" >> /usr/share/snapper/config-templates/default
#echo "TIMELINE_LIMIT_DAILY=\"3\"" >> /usr/share/snapper/config-templates/default
#echo "TIMELINE_LIMIT_WEEKLY=\"3\"" >> /usr/share/snapper/config-templates/default
#echo "TIMELINE_LIMIT_MONTHLY=\"3\"" >> /usr/share/snapper/config-templates/default
#echo "TIMELINE_LIMIT_YEARLY=\"1\"" >> /usr/share/snapper/config-templates/default

#this will create a subvolume


#snapper -c root create-config /
#snapper -c home create-config /home



