package common

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"echoRaft/controller/service"

	"github.com/labstack/echo/v4"
)

var (
	MasterControllerPort          = 65000
	MasterControllerAddr          = "localhost"
	MasterControllerRaftServerAPI = "/api/controller/v1/raftadd"
	MasterControllerKVServerAPI   = "/api/controller/v1/kvadd"
	MasterControllerClientAPI     = "/api/controller/v1/clientadd"
	MetricControllerPort          = 12800
	MetricControllerAddr          = "localhost"
)

type CommonReply struct {
	Ok  bool  `json:"ok,omitempty"`
	Err error `json:"err,omitempty"`
}

func QuickCall(url url.URL, logger echo.Logger, client *http.Client, args interface{}, reply interface{}) bool {
	b, err := json.Marshal(args)
	if err != nil {
		return false
	}
	reqbody := bytes.NewBuffer(b)
	req, err := http.NewRequest("POST", url.String(), reqbody)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error(err)
		return false
	}
	defer resp.Body.Close()

	resbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error(err)
		return false
	}
	// logger.Error(string(resbody))
	if err := json.Unmarshal(resbody, reply); err != nil {
		logger.Error(err)
		return false
	}

	return true
}

func Call(methodName string, args interface{}, reply interface{}, boundMethods map[string]url.URL,
	logger echo.Logger, client *http.Client) bool {
	u := boundMethods[methodName]
	b, err := json.Marshal(args)
	if err != nil {
		return false
	}
	reqbody := bytes.NewBuffer(b)
	req, err := http.NewRequest("POST", u.String(), reqbody)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error(err)
		return false
	}
	defer resp.Body.Close()

	resbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Error(err)
		return false
	}
	// logger.Error(string(resbody))
	if err := json.Unmarshal(resbody, reply); err != nil {
		logger.Error(err)
		return false
	}

	return true
}

func APIFunction(svcMethodFull string, ms map[string]*service.Service) echo.HandlerFunc {
	stringSplit := strings.Split(svcMethodFull, ".")
	service := ms[stringSplit[0]]
	method := service.Methods[stringSplit[1]]
	argtype := method.Type.In(1)
	replyType := method.Type.In(2)
	replyType = replyType.Elem()
	return func(e echo.Context) error {

		args := reflect.New(argtype)
		if err := e.Bind(args.Interface()); err != nil {
			return err
		}
		reply := reflect.New(replyType)

		method.Func.Call([]reflect.Value{service.Rcvr, args.Elem(), reply})

		return e.JSON(http.StatusOK, reply.Interface())
	}
}
