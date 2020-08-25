package controller

import (
	raftData "echoRaft/common/data"
	"echoRaft/controller/common"
	"echoRaft/controller/data"
	"echoRaft/controller/service"
	"echoRaft/raftclient/client"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type ClientController struct {
	data.ControllerData
	kvservers []data.RaftKVClient
	clerk     *client.Clerk

	// New definitions
	// kvControllerClients []*data.RClient
}

func (c *ClientController) Call(svcMethod string, args interface{}, reply interface{}) bool {
	return common.Call(svcMethod, args, reply, c.BoundMethods, c.Ec.Logger, c.Client)

}

func (c *ClientController) InitKVServer(args data.ConnectMasterReply, reply *common.CommonReply) {
	iKvControllers := make([]data.RaftKVClient, len(args.Peers))

	for i, v := range args.Peers {
		kvmethods := make(map[string]url.URL)
		host := strings.Join([]string{v.Addr, strconv.Itoa(v.Port)}, ":")
		for k, url := range c.BoundMethods {
			url.Host = host
			kvmethods[k] = url
		}
		kvClient := data.NewRClient(kvmethods, nil, c.Ec)
		iKvControllers[i] = kvClient
	}
	c.kvservers = iKvControllers
	c.start()
}

func (c *ClientController) API_Get() echo.HandlerFunc {

	return func(e echo.Context) error {
		service := c.Services["Clerk"]
		method := service.Methods["Get"]
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

func (c *ClientController) API_PutAppend() echo.HandlerFunc {

	return func(e echo.Context) error {
		service := c.Services["Clerk"]
		method := service.Methods["PutAppend"]
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

func (c *ClientController) API_InitKVServer() echo.HandlerFunc {
	return func(e echo.Context) error {
		args := &data.ConnectMasterReply{}
		if err := e.Bind(args); err != nil {
			return err
		}

		reply := &common.CommonReply{}
		c.InitKVServer(*args, reply)

		return e.JSON(http.StatusOK, reply)
	}
}

func (c *ClientController) StartEchoService() {
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

func (c *ClientController) Init() {

	c.Ec.GET("/"+c.BoundMethods["Clerk.Get"].Path, c.API_Get())
	c.Ec.POST("/"+c.BoundMethods["Clerk.PutAppend"].Path, c.API_PutAppend())
	c.Ec.POST("/"+c.BoundMethods["ClientController.InitKVServer"].Path, c.API_InitKVServer())

	c.StartEchoService()
}

func (c *ClientController) start() {
	c.clerk = client.MakeClerk(c.kvservers)
	c.Services = service.MakeService(c.clerk, 0)
}

func MakeClientController2(addr string, port int) *ClientController {
	c := &ClientController{}
	c.Ec = echo.New()
	c.Started = &raftData.AtBool{}
	host := strings.Join([]string{addr, strconv.Itoa(port)}, ":")
	baseURL := url.URL{
		Scheme: "http",
		Host:   host,
	}
	c.BoundMethods = make(map[string]url.URL)
	baseURL.Path = "api/client/v1/get"
	c.BoundMethods["Clerk.Get"] = baseURL
	baseURL.Path = "api/client/v1/putappend"
	c.BoundMethods["Clerk.PutAppend"] = baseURL
	baseURL.Path = "api/client/v1/internal/init_kv_servers"
	c.BoundMethods["ClientController.InitKVServer"] = baseURL
	initKVServerURLString := baseURL.String()

	baseURL.Path = "api/kvraft/v1/get"
	c.BoundMethods["RaftKV.Get"] = baseURL
	baseURL.Path = "api/kvraft/v1/putappend"
	c.BoundMethods["RaftKV.PutAppend"] = baseURL

	c.Client = &http.Client{
		Transport:     &http.Transport{},
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       15 * time.Second,
	}
	c.Addr = host

	masterURL := url.URL{
		Scheme: "http",
		Host:   strings.Join([]string{common.MasterControllerAddr, strconv.Itoa(common.MasterControllerPort)}, ":"),
		Path:   common.MasterControllerClientAPI,
	}
	args := data.RaftConnectDetails{
		Addr:          addr,
		Port:          port,
		Id:            0,
		ReplyFullPath: initKVServerURLString,
	}
	common.QuickCall(masterURL, c.Ec.Logger, c.Client, &args, &common.CommonReply{})
	return c
}

func MakeClientController(addr string, kvservers []data.RaftKVClient) *ClientController {
	c := &ClientController{}
	c.Started = &raftData.AtBool{}
	baseURL := url.URL{
		Scheme: "http",
		Host:   addr,
	}
	c.BoundMethods = make(map[string]url.URL)
	baseURL.Path = "api/client/v1/get"
	c.BoundMethods["Clerk.Get"] = baseURL
	baseURL.Path = "api/client/v1/putappend"
	c.BoundMethods["Clerk.PutAppend"] = baseURL
	c.Client = &http.Client{
		Transport:     &http.Transport{},
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       15 * time.Second,
	}
	c.Addr = addr
	c.kvservers = kvservers

	return c
}
