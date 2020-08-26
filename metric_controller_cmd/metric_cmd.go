// Metric controller start up.

// Metric Controller by default runs at localhost:128000, but can be specified.
// A webapp/host can connect to the metric controller via websocket at
// api/metric/v1/rpccount and receive the data which is collected from
// individual raft servers.
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
	portFlag = flag.Int("port", 12800, "Port")
)

func main() {
	metric := controller.MakeMetricController(*hostFlag, *portFlag)
	metric.Init()
	metric.Start()

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
