#! /bin/bash

ROOT_PSW="$(cat /scriptFilesD/resultsD/chosenUserPasswordF)"

lxc exec fd passwd ubuntu <<EOF
$ROOT_PSW
$ROOT_PSW
EOF

lxc file push ./lxdMountServiceF fd/etc/systemd/system/setup-forward-symlinks.service

lxc file push ./lxdSetupForwardF fd/opt/setup-forward-symlinks.sh

lxc ubuntu fd <<EOF

sudo su -

apt update
apt upgrade

apt install -y pulseaudio va-driver-all vdpau-driver-all qtwayland5 at-spi2-core nano dolphin

sed -i "s/; enable-shm = yes/enable-shm = no/g" /etc/pulse/client.conf

#echo export PULSE_SERVER=unix:/tmp/.pulse-native | tee --append /home/ubuntu/.profile

chmod 755 /opt/setup-forward-symlinks.sh

systemctl daemon-reload

systemctl enable setup-forward-symlinks.service

systemctl --no-block start setup-forward-symlinks.service

echo "export WAYLAND_DISPLAY=wayland-0 XDG_SESSION_TYPE=wayland" >> /home/ubuntu/.profile

echo 'if [ "$XDG_SESSION_TYPE" == "wayland" ]; then' >> /home/ubuntu/.profile
echo '    export MOZ_ENABLE_WAYLAND=1' >> /home/ubuntu/.profile
echo 'fi' >> /home/ubuntu/.profiles

EOF