#!/bin/bash
METRIC=cmd/cmd.exe
DIR=metric_controller_cmd/
OPTIND=1
PORT=0
ADDR=""
CMDLINE=""
EXE="metric.exe"
while [ "$1" != "" ]; do
    case $1 in 
        -p | --port )   shift
                        PORT=$1
                        CMDLINE="$CMDLINE -port=$PORT"
                        ;;
        -a | --addr )   shift
                        ADDR="$1"
                        CMDLINE="$CMDLINE -host=$ADDR"
                        ;;
    esac
    shift
done

if [ "$ADDR" == "" ]; then
    echo "Reverting to application default"
fi

if [ $PORT -eq 0 ]; then
    echo "Reverting to application default"
fi

echo
echo
echo "############################"
echo "Building metric"
echo "############################"
echo 
echo
cd $DIR
go build -o $EXE
clear
echo "############################"
echo "Starting master"
echo "############################"
./$EXE $CMDLINE &> metric.out &
clear
echo "############################"
echo "Metric controller Started"
echo "############################"