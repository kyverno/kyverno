#!/bin/bash

# Function to execute etcdctl commands
execute_etcdctl() {
    local key=$1
    local options=$2
    kubectl -n kube-system exec etcd-kind-control-plane -- sh -c \
        "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
        --key /etc/kubernetes/pki/etcd/server.key \
        --cert /etc/kubernetes/pki/etcd/server.crt \
        get $key $options"
}

# Function to extract size and metadata
get_key_info() {
    local key=$1

    local size=$(execute_etcdctl "$key" "--print-value-only" | wc -c)
    local count=$(execute_etcdctl "$key" "--write-out=fields" | grep "Count" | cut -f2 -d':')

    if [ "$count" -ne 0 ]; then
        local versions=$(execute_etcdctl "$key" "--write-out=fields" | grep "Version" | cut -f2 -d':')
    else
        local versions=0
    fi

    # Return size, count, and versions as a string
    echo "$size $count $versions"
}

# Initialize sum
total_size=0
output_file="/tmp/etcdkeys.txt"

# Get list of policy report keys
keys=$(execute_etcdctl "/registry/wgpolicyk8s.io/policyreports" "--prefix --keys-only")

# Process each key
for key in $keys; do
    read size count versions <<< $(get_key_info "$key")
    total=$((size * versions))
    total_size=$((total_size + total))

    # Log to output file
    echo "$total_size $total $size $versions $count $key" >> "$output_file"
done

# Print final total size
echo "The total size for Policy Reports is $total_size bytes."
