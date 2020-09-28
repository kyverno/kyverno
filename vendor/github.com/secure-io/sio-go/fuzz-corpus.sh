#!/bin/sh

if [ -n "$PWD/corpus" ]; then
    rm -rf "$PWD/corpus"
fi
mkdir "$PWD/corpus"

yes | head -c 16 > "$PWD/corpus/0" 
yes | head -c 1024 > "$PWD/corpus/1" 
yes | head -c 65536 > "$PWD/corpus/2" 
yes n | head -c 32768 > "$PWD/corpus/3" 

cat /dev/urandom | head -c 16 > "$PWD/corpus/4" 
cat /dev/urandom | head -c 1024 > "$PWD/corpus/5" 
cat /dev/urandom | head -c 65536 > "$PWD/corpus/6" 
cat /dev/urandom | head -c 524288 > "$PWD/corpus/7" 

cat /dev/urandom | head -c 16 | base64 > "$PWD/corpus/8" 
cat /dev/urandom | head -c 1024 | base64 > "$PWD/corpus/9" 
cat /dev/urandom | head -c 65536 | base64 > "$PWD/corpus/10" 
cat /dev/urandom | head -c 524288 | base64 > "$PWD/corpus/11" 
