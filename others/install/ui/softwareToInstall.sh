#!/bin/sh

functionsStartUi_softwareToInstall () {
    #depends on functionsCommon_log; 
    #non funziona il default, devo creare un nuovo file di appoggio con i valori di default e chiedere prima con yesNo se ripristinarli
    local functionsStartUi_softwareToInstall_resultFileV=$1
    
    #touch $functionsStartUi_softwareToInstall_resultFileV
    echo "" > $functionsStartUi_softwareToInstall_resultFileV

    local functionsStartUi_softwareToInstall_titleV=$2
    local functionsStartUi_softwareToInstall_bodyTextV=$3
    local functionsStartUi_softwareToInstall_menuArgsV=$4
    local functionsStartUi_softwareToInstall_selectedV=${functionsStartUi_softwareToInstall_menuArgsV}Selected

    [[ ! -f $functionsStartUi_softwareToInstall_selectedV ]] && cp -ar $functionsStartUi_softwareToInstall_menuArgsV $functionsStartUi_softwareToInstall_selectedV

    local functionsStartUi_softwareToInstall_returnValueV=
    local functionsStartUi_softwareToInstall_appV=""
    local functionsStartUi_softwareToInstall_iV=

    
    dialog --colors --title "\Z1\Zb$functionsStartUi_softwareToInstall_titleV " --checklist "\Z1$functionsStartUi_softwareToInstall_appV\nSelect a $functionsStartUi_softwareToInstall_bodyTextV " 0 0 0 $(cat $functionsStartUi_softwareToInstall_selectedV) 2> $functionsStartUi_softwareToInstall_resultFileV
    functionsStartUi_softwareToInstall_returnValueV=$?

    functionsStartUi_softwareToInstall_appV="Please try again "

    sed -i 's/on/off/g' $functionsStartUi_softwareToInstall_selectedV

    for functionsStartUi_softwareToInstall_iV in $(cat $functionsStartUi_softwareToInstall_resultFileV) ; do
        sed -i "s/$functionsStartUi_softwareToInstall_iV $functionsStartUi_softwareToInstall_iV off/$functionsStartUi_softwareToInstall_iV $functionsStartUi_softwareToInstall_iV on/g" $functionsStartUi_softwareToInstall_selectedV
    done

    functionsCommon_log "INFO" "$functionsStartUi_softwareToInstall_titleV"
    functionsCommon_log "WARN" "$(cat $functionsStartUi_softwareToInstall_resultFileV)"

    if [[ $functionsStartUi_softwareToInstall_returnValueV == 1 ]] ; then
        cp -ar $functionsStartUi_softwareToInstall_menuArgsV $functionsStartUi_softwareToInstall_selectedV
    fi
    return $functionsStartUi_softwareToInstall_returnValueV
}