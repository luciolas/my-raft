package main

import (
	"echoRaft/controller"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// func initClients() (clients []*controller.ClientController) {
// 	nClients := 1
// 	clients = make([]*controller.ClientController, nClients)
// 	startPort := 6666
// 	for i := 0; i < nClients; i++ {
// 		clients[i] = controller.MakeClientController2("localhost", startPort)
// 		clients[i].Init()
// 		startPort++
// 	}
// 	return
// }

var (
	port = flag.Int("port", 14000, "Port")
	addr = flag.String("addr", "localhost", "[:] IP address or localhost")
)

func main() {
	flag.Parse()
	client := controller.MakeClientController2(*addr, *port)
	client.Init()

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
