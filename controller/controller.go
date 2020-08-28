package controller

import (
	"echoRaft/controller/common"
	"echoRaft/controller/data"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type IController interface {
	Init()
}

type Controller struct {
	nServers             int
	connectedRaftServers int
	connectedKVServers   int
	mu                   sync.Mutex
	IsCreated            bool
	ec                   *echo.Echo
	kvControllers        []*KVRaftController
	clientControllers    []*ClientController
	Stop                 chan struct{}

	metricController *MetricController

	// New definitions
	upgrader      websocket.Upgrader
	wsraftServers []*data.WSConnection
	wsKvServers   []*data.WSConnection

	raftControllerDetails   []data.RaftKVInitConfig
	kvControllerDetails     []data.RaftConnectDetails
	clientControllerDetails chan data.RaftConnectDetails
	controllerClient        *http.Client
}

func (c *Controller) API_Stop() echo.HandlerFunc {

	return func(e echo.Context) error {
		c.Stop <- struct{}{}
		return e.NoContent(http.StatusOK)
	}
}

func (c *Controller) API_AddRaftServer(readyChnl chan<- struct{}) echo.HandlerFunc {
	return func(e echo.Context) error {
		args := &data.RaftKVInitConfig{}
		if err := e.Bind(args); err != nil {
			return err
		}
		c.mu.Lock()
		if c.connectedRaftServers < c.nServers {
			args.RaftConnect.Id = c.connectedRaftServers
			c.raftControllerDetails[c.connectedRaftServers] = *args
			// fmt.Println(*args)
			c.connectedRaftServers++
			if c.connectedRaftServers == c.nServers {
				select {
				case readyChnl <- struct{}{}:
				default:
				}
			}
		}
		c.mu.Unlock()
		return e.JSON(http.StatusOK, &common.CommonReply{})
	}
}

func (c *Controller) API_AddKVServer(readyChnl chan<- struct{}) echo.HandlerFunc {
	return func(e echo.Context) error {
		args := &data.RaftConnectDetails{}

		if err := e.Bind(args); err != nil {
			return err
		}

		c.mu.Lock()
		defer c.mu.Unlock()
		if c.connectedKVServers < c.nServers {
			c.kvControllerDetails[c.connectedKVServers] = *args
			c.connectedKVServers++
			if c.connectedKVServers == c.nServers {
				select {
				case readyChnl <- struct{}{}:
				default:
				}
			}
		}
		return e.JSON(http.StatusOK, &common.CommonReply{})
	}
}

func (c *Controller) API_AddClient() echo.HandlerFunc {
	return func(e echo.Context) error {
		args := &data.RaftConnectDetails{}
		if err := e.Bind(args); err != nil {
			return err
		}
		reply := &common.CommonReply{}
		select {
		case c.clientControllerDetails <- *args:
			reply.Ok = true
		}

		return e.JSON(http.StatusOK, reply)
	}
}

func (c *Controller) API_RemoveServer() echo.HandlerFunc {
	return func(e echo.Context) error {

		return e.NoContent(http.StatusOK)
	}
}

func (c *Controller) API_StopServer() echo.HandlerFunc {
	return func(e echo.Context) error {
		args := data.StopServerArg{}
		if err := e.Bind(&args); err != nil {
			return err
		}
		idx := args.Idx
		kvServerHost := strings.Join([]string{c.kvControllerDetails[idx].Addr, strconv.Itoa(c.kvControllerDetails[idx].Port)}, ":")
		kvURL := url.URL{
			Scheme: "http",
			Host:   kvServerHost,
			Path:   "api/kvraft/v1/internal/stop",
		}
		go common.QuickCall(kvURL, c.ec.Logger, c.controllerClient, &common.CommonReply{}, &common.CommonReply{})
		for _, connection := range c.raftControllerDetails {
			fullHost := strings.Join([]string{connection.RaftConnect.Addr, strconv.Itoa(connection.RaftConnect.Port)}, ":")
			stopUrl := url.URL{
				Scheme: "http",
				Host:   fullHost,
				Path:   "raft/v1/internal/stop",
			}
			reply := &common.CommonReply{}
			go common.QuickCall(stopUrl, c.ec.Logger, c.controllerClient, &args, reply)
		}

		return e.JSON(http.StatusOK, &common.CommonReply{})
	}
}

func (c *Controller) API_StartServer() echo.HandlerFunc {
	return func(e echo.Context) error {
		args := data.StartServerArg{}
		if err := e.Bind(&args); err != nil {
			return err
		}
		idx := args.Idx
		kvServerHost := strings.Join([]string{c.kvControllerDetails[idx].Addr, strconv.Itoa(c.kvControllerDetails[idx].Port)}, ":")
		kvURL := url.URL{
			Scheme: "http",
			Host:   kvServerHost,
			Path:   "api/kvraft/v1/internal/start",
		}
		go common.QuickCall(kvURL, c.ec.Logger, c.controllerClient, &common.CommonReply{}, &common.CommonReply{})
		for _, connection := range c.raftControllerDetails {
			fullHost := strings.Join([]string{connection.RaftConnect.Addr, strconv.Itoa(connection.RaftConnect.Port)}, ":")
			startURL := url.URL{
				Scheme: "http",
				Host:   fullHost,
				Path:   "raft/v1/internal/start",
			}
			reply := &common.CommonReply{}
			go common.QuickCall(startURL, c.ec.Logger, c.controllerClient, &args, reply)
		}

		return e.JSON(http.StatusOK, &common.CommonReply{})
	}
}

func (c *Controller) Init() {
	raftServersReadyChnl := make(chan struct{})
	kvServersReadyChnl := make(chan struct{})

	go c.serversInit(raftServersReadyChnl, kvServersReadyChnl)

	c.ec.POST(common.MasterControllerRaftServerAPI, c.API_AddRaftServer(raftServersReadyChnl))
	c.ec.POST(common.MasterControllerKVServerAPI, c.API_AddKVServer(kvServersReadyChnl))
	c.ec.POST(common.MasterControllerClientAPI, c.API_AddClient())
	c.ec.POST("/api/controller/v1/remove", c.API_RemoveServer())
	c.ec.POST("/api/controller/v1/stop", c.API_StopServer())
	c.ec.POST("/api/controller/v1/start", c.API_StartServer())
	c.Stop = make(chan struct{}, 1)

}

// Serve is blocking
func (c *Controller) Serve() {
	// TODO: restart tries and restart delay
	c.ec.Logger.Fatal(c.ec.Start(":65000"))
}

func (c *Controller) Wait() {
	<-c.Stop
}

func (c *Controller) serveClients() {
	for {
		client := <-c.clientControllerDetails
		clientReplyAPI := client.ReplyFullPath
		url, err := url.Parse(clientReplyAPI)
		if err != nil {
			c.ec.Logger.Error(err)
			continue
		}
		args := &data.ConnectMasterReply{
			Peers: c.kvControllerDetails,
			Ok:    true,
		}
		common.QuickCall(*url, c.ec.Logger, c.controllerClient, args, &common.CommonReply{})
	}
}

func (c *Controller) serversInit(raftreadyChnl <-chan struct{}, kvreadyChnl <-chan struct{}) {
	<-raftreadyChnl
	<-kvreadyChnl
	raftConnectConfig := make([]data.RaftConnectDetails, len(c.raftControllerDetails))
	for i, raft := range c.raftControllerDetails {
		raftConnectConfig[i] = raft.RaftConnect
	}

	// Init raft connection details
	for i, raft := range c.raftControllerDetails {
		raftServerInitPath := raft.RaftConnect.ReplyFullPath
		raftInitURLObj, err := url.Parse(raftServerInitPath)
		if err != nil {
			c.ec.Logger.Error(err)
			continue
		}
		kvInitURLObject, err := url.Parse(raft.KVInitFullPath)
		if err != nil {
			c.ec.Logger.Error(err)
			continue
		}
		raftInitArgs := &data.ConnectMasterReply{
			Peers: raftConnectConfig,
			Ok:    true,
			Id:    i,
		}

		kvInitArgs := c.kvControllerDetails[i]
		common.QuickCall(*kvInitURLObject, c.ec.Logger, c.controllerClient, &kvInitArgs, &common.CommonReply{})
		common.QuickCall(*raftInitURLObj, c.ec.Logger, c.controllerClient, raftInitArgs, &common.CommonReply{})
	}

	// Start raft Websocket connections
	for i, raft := range c.raftControllerDetails {
		raftWSFullPath := raft.RaftStartWSFullPath
		raftStartWSURLObj, err := url.Parse(raftWSFullPath)
		if err != nil {
			c.ec.Logger.Error(err)
			continue
		}
		raftInitArgs := &data.ConnectMasterReply{
			Peers: raftConnectConfig[i+1:],
			Ok:    true,
			Id:    i,
		}
		common.QuickCall(*raftStartWSURLObj, c.ec.Logger, c.controllerClient, raftInitArgs, &common.CommonReply{})
	}

	for _, raft := range c.raftControllerDetails {
		raftServerStartPath := raft.RaftStartFullPath
		raftStartURLObj, err := url.Parse(raftServerStartPath)
		if err != nil {
			c.ec.Logger.Error(err)
			continue
		}
		common.QuickCall(*raftStartURLObj, c.ec.Logger, c.controllerClient, &common.CommonReply{}, &common.CommonReply{})
	}

	// Start KVServers
	for i, kvserver := range c.kvControllerDetails {
		kvServerInitPath := kvserver.ReplyFullPath
		initObj, err := url.Parse(kvServerInitPath)
		if err != nil {
			c.ec.Logger.Error(err)
			continue
		}
		raftInitArgs := c.raftControllerDetails[i].RaftConnect
		common.QuickCall(*initObj, c.ec.Logger, c.controllerClient, &raftInitArgs, &common.CommonReply{})
	}
	go c.serveClients()

}

func NewController(NraftServers int) (controller *Controller) {
	controller = &Controller{
		nServers:                NraftServers,
		clientControllerDetails: make(chan data.RaftConnectDetails, 1),
	}
	controller.controllerClient = &http.Client{
		Transport:     &http.Transport{},
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       15 * time.Second,
	}
	controller.ec = echo.New()
	controller.raftControllerDetails = make([]data.RaftKVInitConfig, controller.nServers)
	controller.kvControllerDetails = make([]data.RaftConnectDetails, controller.nServers)
	controller.upgrader = websocket.Upgrader{
		CheckOrigin: data.COriginAll(),
	}
	controller.Init()

	return
}
