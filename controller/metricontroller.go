package controller

import (
	raftData "echoRaft/common/data"
	"echoRaft/controller/data"
	"echoRaft/controller/service"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var (
	allowedOrigins = map[string]bool{
		"http://localhost:3000": true,
	}

	upgrader = websocket.Upgrader{
		CheckOrigin: data.COriginAll(),
	}

	webHostID_RPCCount = "WebHost.RPCCount"
	rpcID_RPCCount     = "MetricController.Metrics"

	pollTime = 5 * time.Millisecond
)

type MetricController struct {
	data.ControllerData
	kvservers []data.RaftKVClient

	endClients           []*data.WSConnection
	serversWSConnections []*data.WSConnection
	metric               data.AllMetrics
	wsConn               map[string]*websocket.Conn
	metricChangeChnl     chan bool
	mu                   sync.Mutex
}

func (c *MetricController) MetricUpdate(args data.RPCArgs, reply *data.RPCReply) {
	c.mu.Lock()

	_, ok := c.metric.Metrics[args.Idx]

	if !ok {
		c.metric.Metrics[args.Idx] = &data.Metric{Idx: args.Idx}
	}
	switch args.Type {
	case "HB":
		c.metric.Metrics[args.Idx].HBRPC++
		c.metric.Metrics[args.Idx].LeaderIdx = args.RPCFromIdx
	case "Election":
		c.metric.Metrics[args.Idx].ElectionRPC++
	case "Logs":
		c.metric.Metrics[args.Idx].Logs = args.Logs
	case "SuccessHB":
		c.metric.Metrics[args.Idx].SuccessfulHB++
		c.metric.Metrics[args.Idx].LeaderIdx = args.Idx
	default:
	}
	c.mu.Unlock()
	select {
	case c.metricChangeChnl <- true:
	default:
	}
	// fmt.Printf("Metric---> %d: %s\n", args.Idx, args.Type)

}

func (c *MetricController) WSAPI_RPCCount() echo.HandlerFunc {
	return func(e echo.Context) error {
		ws, err := upgrader.Upgrade(e.Response(), e.Request(), nil)
		if err != nil {
			return err
		}
		wsconn := data.NewWsConnection(ws, c.Ec, nil)
		wsconn.StartRPCMode(c.MetricUpdate, []reflect.Value{})
		c.serversWSConnections = append(c.serversWSConnections, wsconn)
		// c.wsConn[rpcID_RPCCount] = ws
		return nil
	}
}

func (c *MetricController) WSWebHost_GetRPCCount() echo.HandlerFunc {
	return func(e echo.Context) error {

		ws, err := upgrader.Upgrade(e.Response(), e.Request(), nil)
		if err != nil {
			c.Ec.Logger.Error(e.Request().Header)
			c.Ec.Logger.Error(err)
			return err
		}
		wsconn := data.NewWsConnection(ws, c.Ec, nil)
		wsconn.StartWriter()
		c.endClients = append(c.endClients, wsconn)
		// c.wsConn[webHostID_RPCCount] = ws
		return nil
	}

}

func (c *MetricController) StartEchoService() {
	go func() {
		if c.Started.Load() {
			return
		}
		c.Started.Set(true)
		defer func() {
			for _, v := range c.serversWSConnections {
				v.Close()
			}
		}()
		defer func() {
			c.Started.Set(false)
		}()
		c.Ec.Logger.Fatal(c.Ec.Start(c.Addr))
	}()
}

func (c *MetricController) Start() {
	c.Services = service.MakeService(c, 0)
}

func (c *MetricController) updateEndClientsOnChange() {
	for {
		select {
		case <-c.metricChangeChnl:
			for _, client := range c.endClients {
				client.Write(c.metric)
			}
		}
	}
}

func (c *MetricController) Init() {
	ec := echo.New()
	c.Ec = ec

	ec.GET("/"+c.BoundMethods[rpcID_RPCCount].Path, c.WSAPI_RPCCount())
	ec.GET("/"+c.BoundMethods[webHostID_RPCCount].Path, c.WSWebHost_GetRPCCount())
	go c.updateEndClientsOnChange()
	c.StartEchoService()
}

func MakeMetricController(addr string, port int) *MetricController {
	c := &MetricController{}
	c.Started = &raftData.AtBool{}
	c.metric = data.AllMetrics{
		Metrics:     make(map[int]*data.Metric),
		LeaderIdxes: make([]int, 0),
	}
	c.endClients = make([]*data.WSConnection, 0)
	c.metricChangeChnl = make(chan bool, 1)
	c.wsConn = make(map[string]*websocket.Conn)
	var addrBuilder strings.Builder
	addrBuilder.WriteString(addr)
	addrBuilder.WriteString(":")
	addrBuilder.WriteString(strconv.FormatInt(int64(port), 10))
	host := addrBuilder.String()
	baseURL := url.URL{
		Scheme: "http",
		Host:   host,
	}
	c.BoundMethods = make(map[string]url.URL)
	baseURL.Path = "api/metric/v1/rpccount"
	c.BoundMethods[rpcID_RPCCount] = baseURL

	baseURL.Path = "api/metric/v1/getrpccount"
	c.BoundMethods[webHostID_RPCCount] = baseURL

	// baseURL.Scheme = "ws"
	// baseURL.Host = "localhost:8080"
	// c.BoundMethods[webHostID_RPCCount] = baseURL
	// c.CreateWS(baseURL, "ServerHost.RPCCount")

	c.Client = &http.Client{
		Transport:     &http.Transport{},
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       15 * time.Second,
	}
	c.Addr = host
	return c
}
