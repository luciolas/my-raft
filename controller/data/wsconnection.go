package data

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"runtime/debug"
	"sync"

	"echoRaft/common/data"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type WSConnection struct {
	mu sync.Mutex
	Ws *websocket.Conn
	Ec *echo.Echo

	raceTester  *data.At32
	writeChnl   chan interface{}
	readChnl    chan interface{}
	readHandler interface{}

	autoReply    *data.AtBool
	autoReplyVal reflect.Value

	actualParameterValues []reflect.Value
	incomingType          reflect.Type
	outgoingType          reflect.Type
	grpcMode              *data.AtBool

	uRunningConnections *data.AtU64
	aRunningConnections []chan interface{}
}

type internalConnection struct {
	MsgType int         `json:"msg_type"`
	Idx     uint64      `json:"idx,omitempty"`
	Args    interface{} `json:"args,omitempty"`
	Reply   interface{} `json:"reply,omitempty"`
}

var (
	READLOGS    = 512
	WRITELOGS   = 128
	CONNECTIONS = 256
)

const (
	NEEDREPLY = iota
	NOTREPLY
	REPLY
)

// Func that get the type of the first parameter of 'method'
func getMethodArgType(method interface{}) (in reflect.Type, out reflect.Type, err error) {
	if method == nil {
		return nil, nil, fmt.Errorf("no method")
	}
	methodType := reflect.TypeOf(method)
	if methodType.Kind() != reflect.Func {
		return nil, nil, fmt.Errorf("not a method")
	}
	if methodType.NumIn() != 2 && methodType.NumIn() != 3 {
		return nil, nil, fmt.Errorf("expected 2 or 3 parameters, found %d", methodType.NumIn())
	}
	firstArgs := methodType.In(methodType.NumIn() - 2)
	secondArgs := methodType.In(methodType.NumIn() - 1)
	if firstArgs.Kind() == reflect.Ptr {
		in = firstArgs.Elem()
	} else {
		in = firstArgs
	}
	if secondArgs.Kind() == reflect.Ptr {
		out = secondArgs.Elem()
	} else {
		out = secondArgs
	}
	return
}

func (wrs *WSConnection) wsReader() {

	method := reflect.ValueOf(wrs.readHandler)
	paraLength := len(wrs.actualParameterValues) + 2
	var parameters []reflect.Value = make([]reflect.Value, paraLength)
	for i, p := range wrs.actualParameterValues {
		parameters[i] = p
	}
	for {
		obj := &internalConnection{}

		// incoming := reflect.New(wrs.incomingType)
		// outgoing := reflect.New(wrs.outgoingType)
		if err := wrs.Ws.ReadJSON(obj); err != nil {
			wrs.Ec.Logger.Fatal(err)
			break
		}
		if !method.IsZero() && obj.MsgType == NEEDREPLY {
			// parameters[paraLength-2] = incoming.Elem()
			// parameters[paraLength-1] = outgoing
			obj.MsgType = REPLY
			argBytes, err := json.Marshal(obj.Args)
			if err != nil {
				wrs.Ec.Logger.Error(err)
				wrs.writeChnl <- obj
				continue
			}
			incoming := reflect.New(wrs.incomingType)
			outgoing := reflect.New(wrs.outgoingType)
			err = json.Unmarshal(argBytes, incoming.Interface())

			if err != nil {
				wrs.Ec.Logger.Error(err)
				wrs.writeChnl <- obj
				continue
			}
			parameters[paraLength-2] = incoming.Elem()
			parameters[paraLength-1] = outgoing
			method.Call(parameters)
			obj.Reply = outgoing.Interface()
			wrs.writeChnl <- obj
		} else {
			// select {
			// case wrs.readChnl <- incoming:
			// default:
			// }
			wrs.mu.Lock()
			runningConnection := wrs.aRunningConnections[obj.Idx]
			wrs.mu.Unlock()
			runningConnection <- obj
		}
	}
}

func (wrs *WSConnection) wsWriter() {
	for {
		select {
		case outgoing := <-wrs.writeChnl:
			if err := wrs.Ws.WriteJSON(outgoing); err != nil {
				wrs.Ec.Logger.Error(err)
				return
			}
		}

	}
}

func (wrs *WSConnection) SetAutoReply(reply interface{}) {
	wrs.autoReply.Set(true)
	wrs.autoReplyVal = reflect.Indirect(reflect.ValueOf(reply))
}

func (wrs *WSConnection) StopAutoReply() {
	wrs.autoReply.Set(false)
}

func (wrs *WSConnection) StartWriter() {

	wrs.raceTester.Add(1)
	if wrs.raceTester.Load() > 2 {
		wrs.Ec.Logger.Fatal(string(debug.Stack()))
	}
	go wrs.wsWriter()
}

func (wrs *WSConnection) StartReader() {

	wrs.raceTester.Add(1)
	if wrs.raceTester.Load() > 2 {
		wrs.Ec.Logger.Fatal(string(debug.Stack()))
	}
	go wrs.wsReader()
}

// Non-blocking write.
func (wrs *WSConnection) Write(arg interface{}) {
	select {
	case wrs.writeChnl <- arg:
	default:
	}
}

func (wrs *WSConnection) WriteMust(arg interface{}) {
	writeObj := &internalConnection{
		Idx:   wrs.uRunningConnections.Load(),
		Args:  arg,
		Reply: nil,
	}
	wrs.uRunningConnections.Add(1)
	wrs.writeChnl <- writeObj
}

func (wrs *WSConnection) ReceiveMust(reply interface{}) {
	reply = <-wrs.readChnl

}

func (wrs *WSConnection) WriteAndReceive(arg interface{}, reply interface{}) {

	if wrs.grpcMode.Load() {
		if wrs.uRunningConnections.Add(1) >= uint64(CONNECTIONS) {
			wrs.uRunningConnections.Set(0)
		}
		writeObj := &internalConnection{
			MsgType: NEEDREPLY,
			Idx:     wrs.uRunningConnections.Load(),
			Args:    arg,
			Reply:   reply,
		}
		wrs.mu.Lock()
		runningConnection := wrs.aRunningConnections[writeObj.Idx]
		wrs.mu.Unlock()
		select {
		case wrs.writeChnl <- writeObj:
		default:
		}
		replyInterface := <-runningConnection
		replyObj := replyInterface.(*internalConnection)
		replybytes, err := json.Marshal(replyObj.Reply)
		if err != nil {
			wrs.Ec.Logger.Error(err)
			return
		}
		if err := json.Unmarshal(replybytes, reply); err != nil {
			wrs.Ec.Logger.Error(err)
			return
		}

	}
}

func (wrs *WSConnection) StartRPCMode(readHandler interface{}, capture []reflect.Value) {
	argType, replyType, err := getMethodArgType(readHandler)
	if err != nil {
		wrs.Ec.Logger.Error(err)
		return
	}
	wrs.incomingType = argType
	wrs.outgoingType = replyType
	wrs.actualParameterValues = capture
	wrs.grpcMode.Set(true)
	wrs.readHandler = readHandler
	wrs.StartReader()
	wrs.StartWriter()
}

func (wrs *WSConnection) Close() {
	wrs.Ws.Close()
	close(wrs.writeChnl)
	close(wrs.readChnl)
	wrs.mu.Lock()
	defer wrs.mu.Unlock()
	for _, conn := range wrs.aRunningConnections {
		select {
		case conn <- struct{}{}:
			close(conn)
		default:
		}
	}
	wrs.uRunningConnections.Set(0)
}

func NewWsConnection(ws *websocket.Conn, ec *echo.Echo, readHandler interface{}) (wrs *WSConnection) {
	aRunningConnections := make([]chan interface{}, CONNECTIONS)
	for i := range aRunningConnections {
		aRunningConnections[i] = make(chan interface{})
	}
	wrs = &WSConnection{
		Ws:                    ws,
		Ec:                    ec,
		readHandler:           readHandler,
		raceTester:            &data.At32{},
		writeChnl:             make(chan interface{}, WRITELOGS),
		readChnl:              make(chan interface{}, READLOGS),
		autoReply:             &data.AtBool{},
		grpcMode:              &data.AtBool{},
		actualParameterValues: make([]reflect.Value, 0),
		uRunningConnections:   &data.AtU64{},
		aRunningConnections:   aRunningConnections,
	}

	return
}

func DialNewWSConnection(bURL url.URL, ec *echo.Echo, requestHeader http.Header, readHandler interface{}) (*WSConnection, error) {
	c, _, err := websocket.DefaultDialer.Dial(bURL.String(), requestHeader)
	if err != nil {
		return nil, err
	}
	wsconnect := NewWsConnection(c, ec, readHandler)
	return wsconnect, nil
}
