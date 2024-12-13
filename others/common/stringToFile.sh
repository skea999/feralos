#!/bin/sh

#Put a string at the end of a file
functionsCommon_stringToFile () {
    #depends on functionsCommon_log

    local string=$1
    local file=$2
    #local ecapedString=$(printf '%s\n' "$string" | sed -e 's/[\/&]/\\&/g')


    functionsCommon_log "INFO" "writing: $string in $file"
    echo "$string" >> $file
    #put an input if you want to dysplay the file
    #functionsCommon_log "WARN" "$(cat $file) "
    echo
}