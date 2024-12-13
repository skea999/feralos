#!/bin/bash

# stores result array in $1
functionsChrootHardwareDetenction_detectGPUVendors () {
    local -A functionsChrootHardwareDetenction_detectGPUVendors_vendorsMapV
    functionsChrootHardwareDetenction_detectGPUVendors_vendorsMapV=( ["8086"]="intel" ["10de"]="nvidia" ["1002"]="ati" ["1022"]="amd" )
    # -n makes it a pointer
    local -n functionsChrootHardwareDetenction_detectGPUVendors_detectedVendorNamesV
    functionsChrootHardwareDetenction_detectGPUVendors_detectedVendorNamesV=$1
    local functionsChrootHardwareDetenction_detectGPUVendors_gpuVendorsV=$(lspci -n | awk '{ print $2, $3 }' | grep -i '^03' | awk '{ print $2 }' | cut -f1 -d ":" )

    local functionsChrootHardwareDetenction_detectGPUVendors_iV=0
    for functionsChrootHardwareDetenction_detectGPUVendors_vendorIdV in $functionsChrootHardwareDetenction_detectGPUVendors_gpuVendorsV; do
        functionsChrootHardwareDetenction_detectGPUVendors_detectedVendorNamesV[$functionsChrootHardwareDetenction_detectGPUVendors_iV]=${functionsChrootHardwareDetenction_detectGPUVendors_vendorsMapV[$functionsChrootHardwareDetenction_detectGPUVendors_vendorIdV]}
        functionsChrootHardwareDetenction_detectGPUVendors_iV=$((functionsChrootHardwareDetenction_detectGPUVendors_iV+1))
    done
}

functionsChrootHardwareDetenction_detectCPUVendor () {
    local functionsChrootHardwareDetenction_detectCPUVendor_cpuVendorV=$(cat /proc/cpuinfo | grep 'vendor_id' | awk '{ print $3 }' | head -1  )
    echo "$functionsChrootHardwareDetenction_detectCPUVendor_cpuVendorV"
}


functionsChrootHardwareDetenction_detectCPUModel () {
    local functionsChrootHardwareDetenction_detectCPUModel_cpuModelV=$(cat /proc/cpuinfo | grep "model name" | cut -f2 -d ":" | sed 's/^ *//g' | head -1  )
    echo "$functionsChrootHardwareDetenction_detectCPUModel_cpuModelV"
}

functionsChrootHardwareDetenction_detectIntelCPUVersion () {
    local functionsChrootHardwareDetenction_detectIntelCPUVersion_inV=$(cat /proc/cpuinfo | grep "model name" | awk '{print $6}' | head -n 1)
    local functionsChrootHardwareDetenction_detectIntelCPUVersion_inA=(${functionsChrootHardwareDetenction_detectIntelCPUVersion_inV//-/ })
    local functionsChrootHardwareDetenction_detectIntelCPUVersion_CPUSeriesV=${functionsChrootHardwareDetenction_detectIntelCPUVersion_inA[1]:0:1}
    return $functionsChrootHardwareDetenction_detectIntelCPUVersion_CPUSeriesV
}

# returns 1 if intel cpu version is broadwell, 0 otherwise
functionsChrootHardwareDetenction_isBroadwell () {
    local -i functionsChrootHardwareDetenction_isBroadwell_CPUVersionV=$(functionsChrootHardwareDetenction_detectIntelCPUVersion)
    local -i functionsChrootHardwareDetenction_isBroadwell_yesV=5
    if [ $functionsChrootHardwareDetenction_isBroadwell_CPUVersionV -eq $functionsChrootHardwareDetenction_isBroadwell_yesV ]; then 
        return 1
    fi
    return 0
}
