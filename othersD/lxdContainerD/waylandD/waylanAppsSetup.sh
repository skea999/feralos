#!/bin/sh

lxc profile create w-fd

cat w-fd.yml | lxc profile edit w-fd

lxc launch ubuntu:22.04 wapps -p default -p w-fd

echo "xhost +local:$(cat /scriptv/scriptv/USR_V)" >> /home/$(cat /scriptv/scriptv/USR_V)/.bash_profile