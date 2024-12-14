#!/bin/bash

#source ./others/common/stringToFile.sh
#Put a string at the end of a file
com_stringToFile () {
    source ./others/common/log.sh

    local string=$1
    local file=$2
    #local ecapedString=$(printf '%s\n' "$string" | sed -e 's/[\/&]/\\&/g')


    com_log "INFO" "writing: $string in $file"
    echo "$string" >> $file
    #put an input if you want to dysplay the file
    #com_log "WARN" "$(cat $file) "
    echo
}