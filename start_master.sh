#!/bin/bash

MASTERDIR=cmd/
OPTIND=1
NSERVERS=3
EXE="master.exe"

while [ "$1" != "" ]; do
    case $1 in
        -n | --nservers )   shift
                            NSERVERS=$1
                            ;;
    esac
    shift
done


echo
echo
echo "############################"
echo "Building Master"
echo "############################"
echo 
echo
cd $MASTERDIR
go build -o $EXE
clear
echo "############################"
echo "Starting master"
echo "############################"
./$EXE -servers $NServers &> master.out &
clear
echo "############################"
echo "Started at :65000"
echo "############################"