package controller

import (
	raftData "echoRaft/common/data"
	"echoRaft/controller/common"
	"echoRaft/controller/data"
	raftkv "echoRaft/kvraft"
	"echoRaft/raft"
	"log"
	"reflect"
	"strconv"
	"strings"

	"echoRaft/controller/service"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
)

type KVRaftController struct {
	data.ControllerData
	kvraft        *raftkv.RaftKV
	persister     *raft.Persister
	raftAPIClient data.RaftKVClient
}

const RAFTKV = "RaftKV"
const Get = "Get"
const PutAppend = "PutAppend"

func (c *KVRaftController) Call(methodName string, args interface{}, reply interface{}) bool {
	return common.Call(methodName, args, reply, c.BoundMethods, c.Ec.Logger, c.Client)
}

func (c *KVRaftController) StartEchoService() {
	go func() {
		if c.Started.Load() {
			return
		}
		c.Started.Set(true)
		defer func() {
			c.Started.Set(false)
		}()
		c.Ec.Logger.Fatal(c.Ec.Start(c.Addr))
	}()
}

func (c *KVRaftController) API_Get() echo.HandlerFunc {

	return func(e echo.Context) error {
		service := c.Services[RAFTKV]
		method := service.Methods[Get]
		argtype := method.Type.In(1)

		args := reflect.New(argtype)
		if err := e.Bind(args.Interface()); err != nil {
			return err
		}
		replyType := method.Type.In(2)
		replyType = replyType.Elem()
		reply := reflect.New(replyType)

		method.Func.Call([]reflect.Value{service.Rcvr, args.Elem(), reply})

		return e.JSON(http.StatusOK, reply.Interface())
	}
}

func (c *KVRaftController) API_PutAppend() echo.HandlerFunc {

	return func(e echo.Context) error {
		service := c.Services[RAFTKV]
		method := service.Methods[PutAppend]
		argtype := method.Type.In(1)

		args := reflect.New(argtype)
		if err := e.Bind(args.Interface()); err != nil {
			return err
		}
		replyType := method.Type.In(2)
		replyType = replyType.Elem()
		reply := reflect.New(replyType)

		method.Func.Call([]reflect.Value{service.Rcvr, args.Elem(), reply})

		return e.JSON(http.StatusOK, reply.Interface())
	}
}

func (c *KVRaftController) API_Apply() echo.HandlerFunc {

	return func(e echo.Context) error {
		service := c.Services[RAFTKV]
		method := service.Methods["Apply"]
		argtype := method.Type.In(1)

		args := reflect.New(argtype)
		if err := e.Bind(args.Interface()); err != nil {
			return err
		}
		replyType := method.Type.In(2)
		replyType = replyType.Elem()
		reply := reflect.New(replyType)

		method.Func.Call([]reflect.Value{service.Rcvr, args.Elem(), reply})

		return e.JSON(http.StatusOK, reply.Interface())
	}
}

func (c *KVRaftController) API_InitRaft() echo.HandlerFunc {
	return func(e echo.Context) error {
		args := data.RaftConnectDetails{}
		if err := e.Bind(&args); err != nil {
			return err
		}
		c.Me = args.Id
		raftBoundMethods := make(map[string]url.URL)
		raftFullAddr := strings.Join([]string{args.Addr, strconv.Itoa(args.Port)}, ":")
		baseURL := url.URL{
			Scheme: "http",
			Host:   raftFullAddr,
			Path:   "raft/v1/agree",
		}
		raftBoundMethods["Raft.Agree"] = baseURL
		baseURL.Path = "raft/v1/get_state"
		raftBoundMethods["Raft.ReturnState"] = baseURL
		raftClient := data.NewRClient(raftBoundMethods, nil, c.Ec)

		c.StartKVRaft(raftClient)
		return e.JSON(http.StatusOK, &common.CommonReply{})
	}
}

func (c *KVRaftController) API_Stop() echo.HandlerFunc {
	return func(e echo.Context) error {
		rclient, ok := c.raftAPIClient.(*data.RClient)
		if ok {
			rclient.Block()
		}
		return e.JSON(http.StatusOK, &common.CommonReply{})
	}
}

func (c *KVRaftController) API_Start() echo.HandlerFunc {
	return func(e echo.Context) error {
		rclient, ok := c.raftAPIClient.(*data.RClient)
		if ok {
			rclient.Unblock()
		}
		return e.JSON(http.StatusOK, &common.CommonReply{})
	}
}

func (c *KVRaftController) Init() {

	c.Ec.POST("/"+c.BoundMethods[RAFTKV+"."+"Get"].Path, c.API_Get())
	c.Ec.POST("/"+c.BoundMethods[RAFTKV+"."+"PutAppend"].Path, c.API_PutAppend())
	c.Ec.POST("/"+c.BoundMethods[RAFTKV+"."+"Apply"].Path, c.API_Apply())
	c.Ec.POST("/"+c.BoundMethods["KVRaftController"+"."+"InitRaft"].Path, c.API_InitRaft())
	c.Ec.POST("/"+c.BoundMethods["KVRaftController"+"."+"Stop"].Path, c.API_Stop())
	c.Ec.POST("/"+c.BoundMethods["KVRaftController"+"."+"Start"].Path, c.API_Start())

	// Call make KVRaftController
	c.StartEchoService()

}

func (c *KVRaftController) StartKVRaft(raftClient data.RaftKVClient) {
	c.kvraft = raftkv.StartKVServer(raftClient, c.Me, c.persister, -1)
	c.raftAPIClient = raftClient

	// Register rpc methods
	c.Services = service.MakeService(c.kvraft, 0)
	if len(c.Services) == 0 {
		log.Fatal("No services found\n")
	}
	// fmt.Printf("%v", c.Services)
}

func MakeKVRaftController(addr string, port int) *KVRaftController {
	c := &KVRaftController{}
	c.Started = &raftData.AtBool{}
	ec := echo.New()
	c.Ec = ec
	fullAddr := strings.Join([]string{addr, strconv.Itoa(port)}, ":")
	baseURL := url.URL{
		Scheme: "http",
		Host:   fullAddr,
	}
	c.BoundMethods = make(map[string]url.URL)
	baseURL.Path = "api/kvraft/v1/get"
	c.BoundMethods[RAFTKV+"."+"Get"] = baseURL

	baseURL.Path = "api/kvraft/v1/putappend"
	c.BoundMethods[RAFTKV+"."+"PutAppend"] = baseURL

	baseURL.Path = "api/kvraft/v1/apply"
	c.BoundMethods[RAFTKV+"."+"Apply"] = baseURL

	baseURL.Path = "api/kvraft/v1/internal/stop"
	c.BoundMethods["KVRaftController"+"."+"Stop"] = baseURL
	baseURL.Path = "api/kvraft/v1/internal/start"
	c.BoundMethods["KVRaftController"+"."+"Start"] = baseURL

	baseURL.Path = "api/kvraft/v1/internal/init_raft"
	c.BoundMethods["KVRaftController"+"."+"InitRaft"] = baseURL
	initRaftURL := baseURL.String()

	c.Client = &http.Client{
		Transport:     &http.Transport{},
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       15 * time.Second,
	}
	c.Addr = fullAddr
	c.persister = &raft.Persister{}

	masterURL := url.URL{
		Scheme: "http",
		Host:   strings.Join([]string{common.MasterControllerAddr, strconv.Itoa(common.MasterControllerPort)}, ":"),
		Path:   common.MasterControllerKVServerAPI,
	}
	args := &data.RaftConnectDetails{
		Addr:          addr,
		Port:          port,
		ReplyFullPath: initRaftURL,
	}
	// data.DialNewWSConnection(masterUrl, ec, "all", c.WSReadInitRaftServer())
	common.QuickCall(masterURL, c.Ec.Logger, c.Client, args, &common.CommonReply{})
	return c
}
