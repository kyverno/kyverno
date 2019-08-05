#!/bin/bash

### To use this script to generate resource:
### ./resource.sh --file=resource.yaml --replica=10

for i in "$@"
do
case $i in
    --file=*)
    file="${i#*=}"
    shift
    ;;
    --replica=*)
    replica="${i#*=}"
    shift
    ;;
esac
done

if [ -z "${file}" ]; then
  echo -e "Please specify '--file' where resource is located."
  exit 1
fi

if [ -z "${replica}" ]; then
  echo -e "Please specify '--replica' of the number of replicas you want to create."
  exit 1
fi

echo "loading resource from ${file}"
RESOURCE=$(cat ${file} | sed -n -e 's/^  name: //p')

echo "generating ${replica} replicas from resource $RESOURCE"

for i in $(seq 1 ${replica})
do
    # echo `cat ${file} | sed "s/name: ${RESOURCE}/name: ${RESOURCE}-${i}/"`
    dstfile=`sed 's/.\{5\}$/-$i&/' <<< "${file}"`
    cat ${file} | sed "s/name: ${RESOURCE}/name: ${RESOURCE}-${i}/" > ${dstfile}
done

