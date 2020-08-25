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

	"github.com/gorilla/websocket"
	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
)

type RaftController struct {
	data.ControllerData
	raftServer  *raft.Raft
	persister   *raft.Persister
	wsConn      map[string]*data.WSConnection
	mu          sync.Mutex
	raftClients []*data.RClient

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

func (rc *RaftController) InitKVServer(args data.RaftConnectDetails, reply *common.CommonReply) {
	fullAddr := strings.Join([]string{args.Addr, strconv.Itoa(args.Port)}, ":")
	baseurl := url.URL{
		Scheme: "http",
		Host:   fullAddr,
		Path:   "api/kvraft/v1/apply",
	}
	rc.BoundMethods["RaftKV.Apply"] = baseurl
}

func (rc *RaftController) InitRaftServer(rcd data.ConnectMasterReply, reply *common.CommonReply) {
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

func (rc *RaftController) StartRaftServer(args common.CommonReply, reply *common.CommonReply) {
	rc.StartRaft2()
}

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

func (rc *RaftController) API_Heartbeat() echo.HandlerFunc {
	return func(e echo.Context) error {
		if sIdx := e.Request().Header.Get("X-Server-Idx"); sIdx != "" {
			ws, err := upgrader.Upgrade(e.Response(), e.Request(), nil)
			if err != nil {
				// TODO :Remove print
				// rc.Ec.Logger.Error(err)
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

func (rc *RaftController) StartEchoService() {
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

func (rc *RaftController) Init() {
	rc.raftControlServices = service.MakeService(rc, 0)

	rc.Ec.GET("/"+rc.BoundMethods["Raft.WSHeartBeat"].Path, rc.API_Heartbeat())

	for _, val := range rc.raftControlServices["RaftController"].SvcMethodName {
		if v, ok := rc.BoundMethods[val]; ok {
			rc.Ec.POST("/"+v.Path, common.APIFunction(val, rc.raftControlServices))
		}
	}
	rc.StartEchoService()
}

func (rc *RaftController) StartRaft2() {
	var iclients []data.RaftKVClient = make([]data.RaftKVClient, len(rc.raftClients))
	for i, v := range rc.raftClients {
		iclients[i] = v
	}
	rc.raftServer = raft.Make(iclients, rc.Me, rc.persister, nil)
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

func (rc *RaftController) StartRaft(raftservers []data.RaftKVClient) {

	rc.raftServer = raft.Make(raftservers, rc.Me, rc.persister, nil)
	rc.Services = service.MakeService(rc.raftServer, 0)
	if len(rc.Services) == 0 {
		log.Fatal("No services found\n")
	}
}

func (rc *RaftController) CreateWS(bURL url.URL, svcName string, readHandler interface{}) *data.WSConnection {
	c, _, err := websocket.DefaultDialer.Dial(bURL.String(), nil)
	if err != nil {
		log.Fatal("dial: ", err)
	}
	wsconnect := data.NewWsConnection(c, rc.Ec, readHandler)
	rc.wsConn[svcName] = wsconnect
	return wsconnect
}

func MakeRaftController2(host string, port int) *RaftController {
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
	c.BoundMethods["RaftController.InitRaftServer"] = baseURL
	initRaftURLString := baseURL.String()

	baseURL.Path = "raft/v1/internal/start_websockets"
	c.BoundMethods["RaftController.StartWebsocket"] = baseURL
	raftWebSocketStartString := baseURL.String()

	baseURL.Path = "raft/v1/internal/start_raft_server"
	c.BoundMethods["RaftController.StartRaftServer"] = baseURL
	raftServerStartString := baseURL.String()

	baseURL.Path = "raft/v1/internal/init_kv_server"
	c.BoundMethods["RaftController.InitKVServer"] = baseURL
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

// func MakeRaftController(addr string, me int, kvraftController *KVRaftController) *RaftController {
// 	c := &RaftController{}
// 	c.Started = &raftData.AtBool{}
// 	c.Ec = echo.New()
// 	c.wsConn = make(map[string]*data.WSConnection)
// 	c.BoundMethods = make(map[string]url.URL)
// 	baseURL := url.URL{
// 		Scheme: "http",
// 		Host:   addr,
// 	}
// 	baseURL.Path = "raft/v1/election"
// 	c.BoundMethods["Raft.RequestVote"] = baseURL

// 	baseURL.Path = "raft/v1/heartbeat"
// 	c.BoundMethods["Raft.HeartBeat"] = baseURL

// 	baseURL.Path = "raft/v1/agree"
// 	c.BoundMethods["Raft.Agree"] = baseURL

// 	baseURL.Path = "raft/v1/internal/stop"
// 	c.BoundMethods["RaftController.StopRaftAPI"] = baseURL

// 	baseURL.Path = "raft/v1/internal/start"
// 	c.BoundMethods["RaftController.StartRaftAPI"] = baseURL

// 	// Special case
// 	baseURL.Host = kvraftController.Addr
// 	baseURL.Path = "api/kvraft/v1/apply"
// 	c.BoundMethods["RaftKV.Apply"] = baseURL

// 	baseURL.Host = strings.Join([]string{common.MetricControllerAddr, strconv.Itoa(common.MetricControllerPort)}, ":")
// 	baseURL.Path = "api/metric/v1/rpccount"
// 	baseURL.Scheme = "ws"
// 	c.BoundMethods["MetricController.Metrics"] = baseURL
// 	c.CreateWS(baseURL, "MetricController.Metrics", nil)

// 	baseURL.Host = strings.Join([]string{common.MasterControllerAddr, strconv.Itoa(common.MasterControllerPort)}, ":")
// 	baseURL.Path = common.MasterControllerRaftServerAPI
// 	baseURL.Scheme = "ws"
// 	c.BoundMethods["Controller.AddServer"] = baseURL
// 	// c.CreateWS(baseURL, "Controller.AddServer", "all", c.WSMasterReadHandler())
// 	c.Client = &http.Client{
// 		Transport:     &http.Transport{},
// 		CheckRedirect: nil,
// 		Jar:           nil,
// 		Timeout:       15 * time.Second,
// 	}

// 	c.Addr = addr
// 	c.Me = me
// 	c.persister = &raft.Persister{}

// 	return c
// }
