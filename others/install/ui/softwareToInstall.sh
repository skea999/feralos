#!/bin/sh

functionsStartUi_softwareToInstall () {
    #depends on functionsCommon_log; 
    #the default does not work; create a new staging file with the default values and ask first with yesNo se ripristinarli
    local resultFile=$1
    
    #touch $resultFile
    echo "" > $resultFile

    local title=$2
    local bodyText=$3
    local menuArgs=$4
    local selected=${menuArgs}Selected

    [[ ! -f $selected ]] && cp -ar $menuArgs $selected

    local returnValue=
    local app=""
    local i=

    
    dialog --colors --title "\Z1\Zb$title " --checklist "\Z1$app\nSelect a $bodyText " 0 0 0 $(cat $selected) 2> $resultFile
    returnValue=$?

    app="Please try again "

    sed -i 's/on/off/g' $selected

    for i in $(cat $resultFile) ; do
        sed -i "s/$i $i off/$i $i on/g" $selected
    done

    functionsCommon_log "INFO" "$title"
    functionsCommon_log "WARN" "$(cat $resultFile)"

    if [[ $returnValue == 1 ]] ; then
        cp -ar $menuArgs $selected
    fi
    return $returnValue
}