#!/bin/bash

## calculate total size for the given object

# read user input for the resource
echo "Enter the resource to caclutate the size:"
read resource

sum=0
for key in `etcdctl get --prefix --keys-only /registry/$resource`
do
  size=`etcdctl get $key --print-value-only | wc -c`
  count=`etcdctl get $key --write-out=fields | grep \"Count\" | cut -f2 -d':'`
  if [ $count -ne 0 ]; then
    versions=`etcdctl get $key --write-out=fields | grep \"Version\" | cut -f2 -d':'`
  else
    versions=0
  fi
  total=$(( $size * $versions))
  sum=$(( $sum + $total ))
  echo $sum $total $size $versions $count $key >> /tmp/etcdkeys.txt
done

echo "The total size for $resource is $sum bytes."