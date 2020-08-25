package data

type RaftLog struct {
	Index int32       `json:"index,omitempty"`
	Term  int32       `json:"term,omitempty"`
	Cmd   interface{} `json:"cmd,omitempty"`
}

type AppendEntriesArgs struct {
	Term            int32     `json:"term,omitempty"`
	LeaderID        int32     `json:"leader_id,omitempty"`
	PrevLog         RaftLog   `json:"prev_log,omitempty"`
	Entries         []RaftLog `json:"entries,omitempty"`
	LeaderCommitIdx int32     `json:"leader_commit_idx,omitempty"`
}

func (rva AppendEntriesArgs) What() string {
	return ""
}

// AppendEntriesArgs struct is the heartbeat payload
type AppendEntriesReply struct {
	Term      int32   `json:"term,omitempty"`
	Success   bool    `json:"success,omitempty"`
	LastEntry RaftLog `json:"last_entry,omitempty"`
}

func (reply *AppendEntriesReply) What() string {
	return ""
}

//
// example RequestVote RPC arguments structure.
//
type RequestVoteArgs struct {
	// Your data here.
	Term int32   `json:"term,omitempty"`
	ID   int32   `json:"id,omitempty"`
	Log  RaftLog `json:"log,omitempty"`
}

func (rva RequestVoteArgs) What() string {
	return ""
}

//
// example RequestVote RPC reply structure.
//
type RequestVoteReply struct {
	// Your data here.
	Term  int32 `json:"term,omitempty"`
	Voted bool  `json:"voted,omitempty"`
}

func (reply *RequestVoteReply) What() string {
	return ""
}

type ApplyMsgReply struct {
	Ok bool `json:"ok,omitempty"`
}

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
//
type ApplyMsg struct {
	Index       int         `json:"index,omitempty"`
	Command     interface{} `json:"command,omitempty"`
	UseSnapshot bool        `json:"use_snapshot,omitempty"` // ignore for lab2; only used in lab3
	Snapshot    []byte      `json:"snapshot,omitempty"`     // ignore for lab2; only used in lab3
}

type AgreeArgs struct {
	Cmd interface{} `json:"cmd,omitempty"`
}

type AgreeReply struct {
	Term     int  `json:"term,omitempty"`
	Index    int  `json:"index,omitempty"`
	IsLeader bool `json:"is_leader,omitempty"`
}

type StateArgs struct {
}

type StateReply struct {
	Term     int  `json:"term"`
	IsLeader bool `json:"is_leader"`
}
