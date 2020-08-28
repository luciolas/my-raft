#!/bin/bash

#Number of servers
NSERVER=0
ADDR=""
PORT=0
MASTERADDR=""
METRICADDR=""
DIR=raft_controller_cmd/
CMDLINE=""
EXE="raft.exe"
if [ $# -lt 3 ]; then
    echo "Must contain at least 3 arguments!"
fi

while [ "$1" != "" ]; do
    case $1 in 
        -n  | --nserver  )  shift
                            NSERVER=$1
                            ;;
        -a | --addr )       shift
                            ADDR=$1
                            ;;
        -sp | --startport )       shift
                            PORT=$1
                            ;;
        -m | --master )     shift
                            MASTERADDR=$1
                            CMDLINE=" $CMDLINE -master=$MASTERADDR "
                            ;;
        -h | --help )       usage
                            exit
                            ;;
        * )                 usage
                            exit 1
    esac
    shift
done

if [ "$ADDR" == "" ]; then 
    echo "No address found! Defaulting to application defaults"
else
    CMDLINE=" $CMDLINE -host=$ADDR "
fi

if [ $PORT -eq 0 ]; then 
    echo "No port found! Defaulting to application defaults"
    # CMDLINE=" $CMDLINE -port=$PORT "
fi

if [ $NSERVER -lt 3 ]; then
    echo "NServer defaults to 3"
    NSERVER=3
fi


######### Build Raft ############
echo "#######################"
echo "Building raft exe"
echo "#######################"
cd $DIR
go build -o $EXE
echo
clear
pwd
echo
echo "#######################"
echo "Done building raft exe"
echo "#######################"
echo 
echo
echo
echo "#######################"
echo "Starting raft servers with cmd $CMDLINE"
echo "#######################"
while [ $NSERVER -gt 0 ]; 
do
    cmd="$CMDLINE"
    cmd="$cmd -port=$PORT"
    ./$EXE $cmd &> $NSERVER.out &
    echo "Start $NServer $cmd"
    PORT=$(($PORT+1))
    NSERVER=$(($NSERVER-1))
done
echo
echo "#######################"
echo "Done starting raft servers"
echo "#######################"
