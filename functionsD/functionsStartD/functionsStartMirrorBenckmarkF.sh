#!/bin/bash

#this is useless because it is integrated in ui but i think that need to be in distribution specific things

local functionsStartMirrorBenckmark_urlV="https://docs.voidlinux.org/xbps/repositories/mirrors/index.html"
local functionsStartMirrorBenckmark_regionsV=$(curl -s $functionsStartMirrorBenckmark_urlV | sed 's/<[^>]*>//g' | grep https:// | sed 's/\([A-Z]\)/ \1/g' | awk '{ s = ""; for (i = 2; i <= NF; i++) s = s $i " "; print s }')
local functionsStartMirrorBenckmark_linksV=$(curl -s $functionsStartMirrorBenckmark_urlV | sed 's/<[^>]*>//g' | grep https:// | sed 's/\([A-Z]\)/ \1/g' | awk '{print $1}')
SPEED_F="speed.txt"
echo "" > $SPEED_F
local functionsStartMirrorBenckmark_bestMirrorV
local functionsStartMirrorBenckmark_TargetPackageV="0ad"

local functionsStartMirrorBenckmark_packageV
 
local functionsStartMirrorBenckmark_pingV
local functionsStartMirrorBenckmark_formattedSpeedV


functionsStartMirrorBenckmark_findPackage () {
    #depends on functionsStartMirrorBenckmark_packageV;
    #not needed unless used for checking
    local functionsStartMirrorBenckmark_findPackage_listingUrlV="https://alpha.de.repo.voidlinux.org/current/musl/"
    local functionsStartMirrorBenckmark_desiredPackageV=$1
    functionsStartMirrorBenckmark_packageV=$(curl -s $functionsStartMirrorBenckmark_findPackage_listingUrlV | sed 's/<[^>]*>//g' | grep $functionsStartMirrorBenckmark_desiredPackageV | grep "x86_64-musl.xbps " | awk '{print $1}' | head -n 1)
}
 
functionsStartMirrorBenckmark_speedTest () {
    local functionsStartMirrorBenckmark_speedTest_rootUrlV=$1
    local functionsStartMirrorBenckmark_speedTest_urlV=${functionsStartMirrorBenckmark_speedTest_rootUrlV}"/current/musl/"${functionsStartMirrorBenckmark_packageV}
    local functionsStartMirrorBenckmark_speedTest_hostV=$(echo $functionsStartMirrorBenckmark_speedTest_rootUrlV | awk -F/ '{print $3}')
    functionsStartMirrorBenckmark_pingV=`ping -c 1 ${functionsStartMirrorBenckmark_speedTest_hostV} | tail -1| awk '{print $4}' | cut -d '/' -f 2 | bc -l`
    local functionsStartMirrorBenckmark_speedTest_speedV=$(curl -qfs -w '%{speed_download}' -o /dev/null --url "$functionsStartMirrorBenckmark_speedTest_urlV")
    functionsStartMirrorBenckmark_formattedSpeedV=$(numfmt --to=iec <<<"$functionsStartMirrorBenckmark_speedTest_speedV")
}
 
functionsStartMirrorBenckmark_backupRestoreIfs () {
    SAVEIFS=$IFS
    IFS=$'\n'
    $1
    IFS=$SAVEIFS
}
 
#functionsStartMirrorBenckmark_backupRestoreIfs 'functionsStartMirrorBenckmark_regionsV=(${functionsStartMirrorBenckmark_regionsV})'
#functionsStartMirrorBenckmark_backupRestoreIfs "functionsStartMirrorBenckmark_linksV=($functionsStartMirrorBenckmark_linksV)"

 
functionsStartMirrorBenckmark_findPackage $functionsStartMirrorBenckmark_TargetPackageV

local functionsStartMirrorBenckmark_iV
for (( functionsStartMirrorBenckmark_iV=0; functionsStartMirrorBenckmark_iV<${#functionsStartMirrorBenckmark_linksV[@]}; functionsStartMirrorBenckmark_iV++ ))
do
    functionsStartMirrorBenckmark_speedTest ${functionsStartMirrorBenckmark_linksV[$functionsStartMirrorBenckmark_iV]}
    region=$(echo ${functionsStartMirrorBenckmark_regionsV[$functionsStartMirrorBenckmark_iV]} | sed 's/ //g')
    echo -e "${functionsStartMirrorBenckmark_linksV[$functionsStartMirrorBenckmark_iV]}\t$functionsStartMirrorBenckmark_formattedSpeedV\t$functionsStartMirrorBenckmark_pingV\t${functionsStartMirrorBenckmark_regionsV}" >> $SPEED_F
    echo "Speed for ${functionsStartMirrorBenckmark_linksV[$functionsStartMirrorBenckmark_iV]} is: ${functionsStartMirrorBenckmark_formattedSpeedV}B/s ping: ${functionsStartMirrorBenckmark_pingV}ms Region: $functionsStartMirrorBenckmark_regionsV"
done
 
#sort file by speed
sort -r -n -k 2 -o $SPEED_F $SPEED_F 
bestMirror=$(cat $SPEED_F | awk '{print $1}' | head -n 1)
echo "Best mirror is: $functionsStartMirrorBenckmark_bestMirrorV"
 
cat $SPEED_F | awk '{print $1 " " $2 "B/s-ping:" $3 "ms_" $4}' | head -n -1 > ${SPEED_F}"_new"
cat ${SPEED_F}"_new" > $SPEED_F
rm ${SPEED_F}"_new"
 