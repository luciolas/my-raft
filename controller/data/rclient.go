package data

import (
	commonData "echoRaft/common/data"
	"echoRaft/controller/common"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
)

type RClient struct {
	client       *http.Client
	boundMethods map[string]url.URL
	WsMethods    map[string]*WSConnection
	e            *echo.Echo
	isBlocked    *commonData.AtBool
}

func (c *RClient) Call(methodName string, args interface{}, reply interface{}) bool {
	if c.isBlocked.Load() {
		return false
	}
	return common.Call(methodName, args, reply, c.boundMethods, c.e.Logger, c.client)
}

func (c *RClient) CallWS(methodName string, args interface{}, reply interface{}) bool {
	if c.isBlocked.Load() {
		return false
	}
	wsconn, wsconnOk := c.WsMethods[methodName]
	if wsconnOk {
		wsconn.WriteAndReceive(args, reply)
		return true
	}
	return false
}

func (c *RClient) Block() {
	c.isBlocked.Set(true)
}

func (c *RClient) Unblock() {
	c.isBlocked.Set(false)
}

func (c *RClient) IsBlocked() bool {
	return c.isBlocked.Load()
}

func NewRClient(bound map[string]url.URL, wsmethods map[string]*WSConnection, e *echo.Echo) (client *RClient) {
	c := &http.Client{
		Transport:     &http.Transport{},
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       15 * time.Second,
	}
	client = &RClient{
		client:       c,
		boundMethods: bound,
		e:            e,
		WsMethods:    wsmethods,
		isBlocked:    &commonData.AtBool{},
	}
	return
}
