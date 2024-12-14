#!/bin/bash

#source ./others/install/ui/softwareToInstall.sh

ins_ui_softwareToInstall () {
    source ./others/common/log.sh

    #non funziona il default, devo creare un nuovo file di appoggio con i valori di default e chiedere prima con yesNo se ripristinarli
    local resultFile=$1
    
    #touch $resultFile
    results=

    local title=$2
    local bodyText=$3
    local menuArgs=$4
    local selected=${menuArgs}Selected

    [[ ! -f $selected ]] && cp -ar $menuArgs $selected

    local returnValue=
    local app=""
    local i=

    
    results=$(dialog --colors --title "\Z1\Zb$title " --checklist "\Z1$app\nSelect a $bodyText " 0 0 0 $(cat $selected) )
    returnValue=$?

    app="Please try again "

    sed -i 's/on/off/g' $selected

    for i in $(cat $resultFile) ; do
        sed -i "s/$i $i off/$i $i on/g" $selected
    done

    com_log "INFO" "$title"
    com_log "WARN" "$(cat $resultFile)"

    if [[ $returnValue == 1 ]] ; then
        cp -ar $menuArgs $selected
    fi
    return $returnValue
}