package main

import (
	"echoRaft/controller"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var (
	hostFlag = flag.String("host", "localhost", "IP address host")
	portFlag = flag.Int("port", 1255, "Port")

	// TODO: Enable user to put in the master and metric addresses.
	masterFlag = flag.String("master", "localhost:65000", "master controller host:port")
	metricFlag = flag.String("metric", "localhost:12800", "metric controller host:port")
)

func main() {
	flag.Parse()

	if *masterFlag == "" {
		*masterFlag = "localhost:65000"
	}
	if *metricFlag == "" {
		*metricFlag = "localhost:12800"
	}
	fmt.Println(*portFlag)
	raftServer := controller.MakeRaftController2(*hostFlag, *portFlag)
	raftServer.Init()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	// listen to sig kill ,terminate, abort and interrupt
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGKILL, syscall.SIGABRT, syscall.SIGTERM)
	// Kill signal wait function
	go func() {
		sig := <-sigs
		fmt.Println(sig)
		done <- true
	}()

	<-done
}
