package controller

import (
	raftData "echoRaft/common/data"
	"echoRaft/controller/common"
	"echoRaft/controller/data"
	"echoRaft/controller/service"
	"echoRaft/raft"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
)

type RaftController struct {
	data.ControllerData
	raftServer *raft.Raft
	persister  *raft.Persister
	wsConn     map[string]*data.WSConnection
	mu         sync.Mutex

	raftClients  []*data.RClient
	iraftClients []data.RaftKVClient

	// Stores methods that are readily to be bound to an API service
	raftControlServices map[string]*service.Service
}

func (rc *RaftController) StopRaftAPI(args data.StopServerArg, reply *common.CommonReply) {
	if args.Idx == rc.Me {
		for _, v := range rc.raftClients {
			v.Block()
		}
	} else {
		rc.raftClients[args.Idx].Block()
	}
	reply.Ok = true
}

func (rc *RaftController) StartRaftAPI(args data.StartServerArg, reply *common.CommonReply) {
	if args.Idx == rc.Me {
		for _, v := range rc.raftClients {
			v.Unblock()
		}
	} else if !rc.raftClients[rc.Me].IsBlocked() {
		rc.raftClients[args.Idx].Unblock()
	}
}

//InitKVServerInfo initialize the KV server connection information (Does not connect to it yet)
func (rc *RaftController) InitKVServerInfo(args data.RaftConnectDetails, reply *common.CommonReply) {
	fullAddr := strings.Join([]string{args.Addr, strconv.Itoa(args.Port)}, ":")
	baseurl := url.URL{
		Scheme: "http",
		Host:   fullAddr,
		Path:   "api/kvraft/v1/apply",
	}
	rc.BoundMethods["RaftKV.Apply"] = baseurl
}

// InitRaftPeers initialize peers' information
func (rc *RaftController) InitRaftPeers(rcd data.ConnectMasterReply, reply *common.CommonReply) {
	rc.Me = rcd.Id
	// Init an interface that has Call
	connectionDetails := rcd.Peers
	var clients []*data.RClient = make([]*data.RClient, len(connectionDetails))
	for _, v := range connectionDetails {
		i := v.Id
		var mBaseURL map[string]url.URL = make(map[string]url.URL)
		if i != rc.Me {
			fullAddr := strings.Join([]string{v.Addr, strconv.Itoa(v.Port)}, ":")
			for k, url := range rc.BoundMethods {
				url.Host = fullAddr
				mBaseURL[k] = url
			}
		} else {
			mBaseURL = rc.BoundMethods
		}

		clients[i] = data.NewRClient(mBaseURL, make(map[string]*data.WSConnection), rc.Ec)
	}
	rc.raftClients = clients

}

// StartRaftServer begins the Raft heartbeat connection
func (rc *RaftController) StartRaftServer(args common.CommonReply, reply *common.CommonReply) {
	rc.StartRaft()
}

func (rc *RaftController) connectToMetricController() {
	metricControllerURL := url.URL{}
	metricControllerURL.Host = strings.Join([]string{common.MetricControllerAddr, strconv.Itoa(common.MetricControllerPort)}, ":")
	metricControllerURL.Path = "api/metric/v1/rpccount"
	metricControllerURL.Scheme = "ws"
	wsconn, err := data.DialNewWSConnection(metricControllerURL, rc.Ec, nil, nil)
	if err != nil {
		rc.Ec.Logger.Error(err)
	} else {
		rc.raftClients[rc.Me].WsMethods["MetricController.Metrics"] = wsconn
	}
}

// StartWebsocket will initialize its own ID (from Master controller) and initialize connection
// with peers
func (rc *RaftController) StartWebsocket(rcd data.ConnectMasterReply, reply *common.CommonReply) {
	headers := make(http.Header)
	headers.Add("X-Server-Idx", strconv.Itoa(rc.Me))
	for _, v := range rcd.Peers {
		i := v.Id
		if i > rc.Me {
			fullAddr := strings.Join([]string{v.Addr, strconv.Itoa(v.Port)}, ":")
			wsHeartBeatURL := url.URL{
				Scheme: "ws",
				Host:   fullAddr,
				Path:   rc.BoundMethods["Raft.WSHeartBeat"].Path,
			}
			wsconn, err := data.DialNewWSConnection(wsHeartBeatURL, rc.Ec, headers, nil)
			if err != nil {
				rc.Ec.Logger.Error(err)
			} else {
				rc.raftClients[i].WsMethods["Raft.WSHeartBeat"] = wsconn
			}
		}
	}

	go rc.connectToMetricController()
}

// API_WSInitPeerConn Init a receiving WS handshake and upgrades it
func (rc *RaftController) API_WSInitPeerConn() echo.HandlerFunc {
	return func(e echo.Context) error {
		if sIdx := e.Request().Header.Get("X-Server-Idx"); sIdx != "" {
			ws, err := upgrader.Upgrade(e.Response(), e.Request(), nil)
			if err != nil {
				return err
			}
			idx, err := strconv.Atoi(sIdx)
			if err != nil {
				return err
			}
			rc.raftClients[idx].WsMethods["Raft.WSHeartBeat"] = data.NewWsConnection(ws, rc.Ec, nil)
		} else {
			rc.Ec.Logger.Error(e.String(http.StatusBadRequest, "unidentified server"))
			return e.String(http.StatusBadRequest, "unidentified server")
		}
		return nil
	}
}

func (rc *RaftController) startEchoService() {
	p := prometheus.NewPrometheus("echo", nil)
	p.Use(rc.Ec)
	go func(rc *RaftController) {
		if rc.Started.Load() {
			return
		}
		rc.Started.Set(true)
		defer func() {
			for _, val := range rc.wsConn {
				val.Ws.Close()
			}
		}()

		defer func() {
			rc.Started.Set(false)
		}()
		rc.Ec.Logger.Fatal(rc.Ec.Start(rc.Addr))
	}(rc)
}

// Init starts an Echo service and serves (non-blocking)
func (rc *RaftController) Init() {
	rc.raftControlServices = service.MakeService(rc, 0)

	rc.Ec.GET("/"+rc.BoundMethods["Raft.WSHeartBeat"].Path, rc.API_WSInitPeerConn())

	for _, val := range rc.raftControlServices["RaftController"].SvcMethodName {
		if v, ok := rc.BoundMethods[val]; ok {
			rc.Ec.POST("/"+v.Path, common.APIFunction(val, rc.raftControlServices))
		}
	}
	rc.startEchoService()
}

// StartRaft will create an instance of the raft logic and begin operations
func (rc *RaftController) StartRaft() {
	rc.iraftClients = make([]data.RaftKVClient, len(rc.raftClients))
	for i, v := range rc.raftClients {
		rc.iraftClients[i] = v
	}
	rc.raftServer = raft.Make(rc.iraftClients, rc.Me, rc.persister, nil)
	rc.Services = service.MakeService(rc.raftServer, 0)
	if len(rc.Services) == 0 {
		log.Fatal("No services found\n")
	}
	raftServices := rc.Services["Raft"]
	for _, val := range raftServices.SvcMethodName {
		if v, ok := rc.BoundMethods[val]; ok {
			rc.Ec.POST("/"+v.Path, common.APIFunction(val, rc.Services))
		}
	}
	for i, client := range rc.raftClients {
		if i != rc.Me {
			client.WsMethods["Raft.WSHeartBeat"].StartRPCMode(
				raftServices.Methods["HeartBeat"].Func.Interface(),
				[]reflect.Value{raftServices.Rcvr})
		}
	}
	metricws, ok := rc.raftClients[rc.Me].WsMethods["MetricController.Metrics"]
	if ok {
		metricws.StartRPCMode(
			raftServices.Methods["SendMetrics"].Func.Interface(),
			[]reflect.Value{raftServices.Rcvr})
	}

}

func MakeRaftController(host string, port int) *RaftController {
	addr := strings.Join([]string{host, strconv.Itoa(port)}, ":")
	c := &RaftController{}
	c.Started = &raftData.AtBool{}
	c.Ec = echo.New()
	c.wsConn = make(map[string]*data.WSConnection)
	c.BoundMethods = make(map[string]url.URL)
	c.raftControlServices = make(map[string]*service.Service)
	baseURL := url.URL{
		Scheme: "http",
		Host:   addr,
	}
	baseURL.Path = "raft/v1/election"
	c.BoundMethods["Raft.RequestVote"] = baseURL

	baseURL.Path = "raft/v1/heartbeat"
	c.BoundMethods["Raft.HeartBeat"] = baseURL

	baseURL.Path = "raft/v1/agree"
	c.BoundMethods["Raft.Agree"] = baseURL
	baseURL.Path = "raft/v1/get_state"
	c.BoundMethods["Raft.ReturnState"] = baseURL

	baseURL.Path = "raft/v1/internal/init_raft_server"
	c.BoundMethods["RaftController.InitRaftPeers"] = baseURL
	initRaftURLString := baseURL.String()

	baseURL.Path = "raft/v1/internal/start_websockets"
	c.BoundMethods["RaftController.StartWebsocket"] = baseURL
	raftWebSocketStartString := baseURL.String()

	baseURL.Path = "raft/v1/internal/start_raft_server"
	c.BoundMethods["RaftController.StartRaftServer"] = baseURL
	raftServerStartString := baseURL.String()

	baseURL.Path = "raft/v1/internal/init_kv_server"
	c.BoundMethods["RaftController.InitKVServerInfo"] = baseURL
	initKVServerString := baseURL.String()

	baseURL.Path = "raft/v1/internal/stop"
	c.BoundMethods["RaftController.StopRaftAPI"] = baseURL

	baseURL.Path = "raft/v1/internal/start"
	c.BoundMethods["RaftController.StartRaftAPI"] = baseURL

	baseURL.Path = "raft/v1/ws/heartbeat"
	c.BoundMethods["Raft.WSHeartBeat"] = baseURL

	c.Client = &http.Client{
		Transport:     &http.Transport{},
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       15 * time.Second,
	}

	c.Addr = addr
	c.persister = &raft.Persister{}

	baseURL.Host = strings.Join([]string{common.MasterControllerAddr, strconv.Itoa(common.MasterControllerPort)}, ":")
	baseURL.Path = common.MasterControllerRaftServerAPI
	baseURL.Scheme = "http"
	// Tell master controller we are ready
	cargs := data.RaftKVInitConfig{
		RaftConnect: data.RaftConnectDetails{
			Addr:          host,
			Port:          port,
			Id:            0,
			ReplyFullPath: initRaftURLString,
		},
		KVInitFullPath:      initKVServerString,
		RaftStartFullPath:   raftServerStartString,
		RaftStartWSFullPath: raftWebSocketStartString,
	}

	common.QuickCall(baseURL, c.Ec.Logger, c.Client, &cargs, &common.CommonReply{})

	return c
}
