package data

import (
	raftData "echoRaft/common/data"
	"echoRaft/controller/service"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
)

type Callable interface {
	What() string
}

type RaftKVClient interface {
	Call(methodName string, arg interface{}, reply interface{}) bool
	CallWS(methodName string, arg interface{}, reply interface{}) bool
}

type ControllerData struct {
	Ec           *echo.Echo
	Client       *http.Client
	Me           int
	Addr         string
	BoundMethods map[string]url.URL
	Services     map[string]*service.Service
	Started      *raftData.AtBool
}

type CreateConfig struct {
	StartPort int `json:"start_port,omitempty"`
	RaftCount int `json:"raft_count,omitempty"`
	// Add more config here
}

type RaftConnectDetails struct {
	Addr          string `json:"addr"`
	Port          int    `json:"port"`
	Id            int    `json:"id"`
	ReplyFullPath string `json:"reply_full_path,omitempty"`
}

type RaftKVInitConfig struct {
	RaftConnect         RaftConnectDetails `json:"raft_connect,omitempty"`
	RaftStartWSFullPath string             `json:"raft_start_ws_full_path,omitempty"`
	RaftStartFullPath   string             `json:"raft_start_full_path,omitempty"`
	KVInitFullPath      string             `json:"kv_init_full_path,omitempty"`
}

type ConnectMasterReply struct {
	Peers []RaftConnectDetails `json:"peers"`
	Ok    bool                 `json:"ok"`
	Id    int                  `json:"id"`
}
