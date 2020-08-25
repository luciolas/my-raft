package main

import (
	"echoRaft/config"
	"echoRaft/controller"
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

func main() {
	con := controller.NewController(config.NRaftServers)
	metric := controller.MakeMetricController(12800)
	metric.Init()
	metric.Start()
	con.Init()

	go con.Serve()

	go func() {
		initRaftServers()
		initKVServers()
		initClients()
	}()

	// _ = initRaftServers()
	// _ = initKVServers()
	// _ = initClients()

	con.Wait()

}
