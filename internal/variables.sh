#! /bin/sh


######### disks where to install alpine, find it with "fdsik -l"##########
disk1="/dev/vda"
disk2="/dev/vdb"

############user and root passwords############
password="helloAlpine3.23"


############common#########
chrootDir="/mnt"
mirror="https://dl-cdn.alpinelinux.org/alpine"
arch="x86_64"
branch="latest-stable"
