#!/bin/bash

DIR=raftclient/
N=1
START_PORT=14000
EXE=client.exe
ADDR=""
CMDLINE=""
while [ "$1" != "" ]; do
    case $1 in
        -n | --nclients )   shift
                            N=$1
                            ;;
        -sp | --port )       shift
                            START_PORT=$1
                            ;;
        -a | --addr )       shift
                            ADDR=$1
                            CMDLINE="$CMDLINE -addr=$ADDR"
                            ;;
    esac
    shift
done

if [ "$ADDR" == "" ]; then
    echo "Using default localhost host..."
fi

if [ $N -le 0 ]; then
    echo "Defaulting client number to 1"
    N=1
fi

echo "##################"
echo "Building client..."
echo "##################"
cd $DIR
go build -o $EXE
clear
echo "##################"
echo "Done building client..."
echo "Starting client"
echo "##################"
sp=$START_PORT
n=$N
while [ $n -gt 0 ]; do
    cmd_line="$CMDLINE -port=$sp"
    ./$EXE $cmd_line &> $n.out &
    sp=$(($sp+1))
    n=$(($n-1))
done