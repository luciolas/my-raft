package data

import (
	"echoRaft/controller/common"
	raftData "echoRaft/raft/data"
	"net/http"
)

type RPCArgs struct {
	Idx        int                `json:"idx"`
	RPCFromIdx int                `json:"rpc_from_idx"`
	Logs       []raftData.RaftLog `json:"logs,omitempty"`
	Type       string             `json:"type,omitempty"`
	VotedFor   int                `json:"voted_for"`
	LeaderIdx  int                `json:"leader_idx"`
}

type AllMetrics struct {
	Metrics     map[int]*Metric `json:"metrics"`
	LeaderIdxes []int           `json:"leader_idxes"`
}

type Metric struct {
	Idx          int                `json:"idx"`
	HBRPC        int                `json:"hbrpc"`
	SuccessfulHB int                `json:"successful_hb"`
	ElectionRPC  int                `json:"election_rpc"`
	Logs         []raftData.RaftLog `json:"logs,omitempty"`
	LeaderIdx    int                `json:"leader_idx"`
}

type RPCReply struct {
	common.CommonReply
}

func COrigin(allowedOrigins map[string]bool) func(*http.Request) bool {
	return func(r *http.Request) bool {
		allowed, ok := allowedOrigins[r.Header.Get("Origin")]
		if allowed && ok {
			return true
		}
		return false
	}
}

func COriginAll() func(*http.Request) bool {
	return func(r *http.Request) bool {
		return true
	}
}
