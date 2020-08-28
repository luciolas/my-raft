// Master controller

// The master controller is a simple, centralised orchestrator to assist in
// collecting raft information and re-disseminate them to every other server.
// It also manages the KV servers and help them to connect to their
// partner raft servers.

//(Not implemented)
// It also serves as the main orchestrator for add,remove and partition servers
//(Not implemented)

package main

import (
	"echoRaft/config"
	"echoRaft/controller"
	"flag"
)

func initRaftServers() (raftServers []*controller.RaftController) {
	raftServers = make([]*controller.RaftController, config.NRaftServers)
	startPort := config.RaftServerStartPort
	for i := 0; i < config.NRaftServers; i++ {
		raftServers[i] = controller.MakeRaftController2("localhost", startPort)
		raftServers[i].Init()
		startPort++
	}
	return
}

func initKVServers() (kvservers []*controller.KVRaftController) {
	kvservers = make([]*controller.KVRaftController, config.NRaftServers)
	startPort := config.KVServerStartPort
	for i := 0; i < config.NRaftServers; i++ {
		kvservers[i] = controller.MakeKVRaftController("localhost", startPort)
		kvservers[i].Init()
		startPort++
	}
	return
}

func initClients() (clients []*controller.ClientController) {
	nClients := 1
	clients = make([]*controller.ClientController, nClients)
	startPort := 6666
	for i := 0; i < nClients; i++ {
		clients[i] = controller.MakeClientController2("localhost", startPort)
		clients[i].Init()
		startPort++
	}
	return
}

var (
	NServers = flag.Int("servers", 3, "number of raft servers")
)

func main() {
	flag.Parse()
	con := controller.NewController(*NServers)
	// metric := controller.MakeMetricController("localhost", 12800)
	// metric.Init()
	// metric.Start()
	con.Init()

	// go con.Serve()
	con.Serve()

	// go func() {
	// 	initRaftServers()
	// 	initKVServers()
	// 	initClients()
	// }()
	// _ = initRaftServers()
	// _ = initKVServers()
	// _ = initClients()

	// con.Wait()

	// sigs := make(chan os.Signal, 1)
	// done := make(chan bool, 1)

	// // listen to sig kill ,terminate, abort and interrupt
	// signal.Notify(sigs, syscall.SIGINT, syscall.SIGKILL, syscall.SIGABRT, syscall.SIGTERM)
	// // Kill signal wait function
	// go func() {
	// 	sig := <-sigs
	// 	fmt.Println(sig)
	// 	done <- true
	// }()

	// <-done

}
