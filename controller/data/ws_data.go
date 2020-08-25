package data

import (
	raftData "echoRaft/raft/data"
)

type WSHBData struct {
	Idx   int                          `json:"idx,omitempty"`
	Arg   *raftData.AppendEntriesArgs  `json:"arg,omitempty"`
	Reply *raftData.AppendEntriesReply `json:"reply,omitempty"`
}
