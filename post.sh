#!/bin/bash

KEY=$1
VAL=$2
OP=$3
HOSTURL="http://localhost:6666"
PUTAPPENDAPIURL="/api/client/v1/putappend"
GETAPIURL="/api/client/v1/get"
RESTMETHOD="POST"
APIURL=""

case $OP in
    "A" | "a" | "append" | "Append" | "APPEND")
        OP="Append"
        APIURL="$PUTAPPENDAPIURL"
        ;;
    "put" | "PUT" | "Put")
        OP="Put"
        APIURL="$PUTAPPENDAPIURL"
        ;;
    "g" | "get" | "Get" | "GET")
        RESTMETHOD="GET"
        OP="Get"
        APIURL="$GETAPIURL"
        ;;
    *)
        echo -n "unknown rest method"
        exit 2
        ;;
esac


DATA="{\"key\" : \"$KEY\", \"value\":\"$VAL\", \"op\":\"$OP\"}"

curl -X $RESTMETHOD -H "Content-Type:application/json" $HOSTURL$APIURL -d "$DATA"

