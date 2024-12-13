#!/bin/sh

#Put a string at the end of a file
functionsCommon_stringToFile () {
    #depends on functionsCommon_log

    local functionsCommon_stringToFile_stringV=$1
    local functionsCommon_stringToFile_fileV=$2
    #local functionsCommon_stringToFile_ecapedStringV=$(printf '%s\n' "$functionsCommon_stringToFile_stringV" | sed -e 's/[\/&]/\\&/g')


    functionsCommon_log "INFO" "writing: $functionsCommon_stringToFile_stringV in $functionsCommon_stringToFile_fileV"
    echo "$functionsCommon_stringToFile_stringV" >> $functionsCommon_stringToFile_fileV
    #put an input if you want to dysplay the file
    #functionsCommon_log "WARN" "$(cat $functionsCommon_stringToFile_fileV) "
    echo
}