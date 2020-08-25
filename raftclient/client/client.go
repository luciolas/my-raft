package client

import (
	"crypto/rand"
	cData "echoRaft/controller/data"
	"echoRaft/raftclient/common"
	clientdto "echoRaft/raftclient/dto"
	"math/big"
	"strconv"
	"strings"
)

var CONTEXTSEPARATOR = ":"

type Clerk struct {
	servers []cData.RaftKVClient
	// You will have to modify this struct.

	lastLeader int
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []cData.RaftKVClient) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.lastLeader = -1
	return ck
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("RaftKV.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) Get(getArgs clientdto.ClientGetArg, clientReply *clientdto.ClientGetReply) {

	// You will have to modify this function.
	var cmd strings.Builder
	cmd.WriteString("Get")
	cmd.WriteString(common.OPSeparator)
	cmd.WriteString(getArgs.Key)
	raftKVGetArgs := &common.GetArgs{
		Op: common.Op{
			ContextId: strconv.FormatInt(nrand(), 36) + CONTEXTSEPARATOR + strconv.FormatInt(nrand(), 12),
			Cmd:       cmd.String(),
		},
	}

	// If there is a lastleader, we will try that first,
	// if there are no last leader, we need to try all
	// servers and set the success one as lastleader
	result := func() string {
		for {
			if ck.lastLeader < 0 {
				for i, s := range ck.servers {
					reply := &common.GetReply{}
					ok := s.Call("RaftKV.Get", raftKVGetArgs, reply)
					if reply.Err == common.ErrNoKey {
						// log error
						return "No key"
					}
					if !ok || reply.WrongLeader {
						continue
					}
					ck.lastLeader = i
					if reply.Err == common.OK {
						common.DPrintf("Success get: contextID: %s", raftKVGetArgs.Op.ContextId)
						return reply.Value
					}
				}
			} else {
				reply := &common.GetReply{}
				ok := ck.servers[ck.lastLeader].Call("RaftKV.Get", raftKVGetArgs, reply)
				if reply.Err == common.ErrNoKey {
					// log error
					return "No key"
				}
				if !ok || reply.WrongLeader {
					ck.lastLeader = -1
				} else if reply.Err == common.OK {
					common.DPrintf("Success get: contextID: %s", raftKVGetArgs.Op.ContextId)
					return reply.Value
				}
			}
		}
	}()

	clientReply.Key = getArgs.Key
	clientReply.Value = result
}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("RaftKV.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) PutAppend(putappendArgs clientdto.ClientPutAppendArg, reply *clientdto.ClientPutAppendReply) {
	// You will have to modify this function.
	if putappendArgs.Op != "Put" && putappendArgs.Op != "Append" {
		return
	}
	var cmd strings.Builder
	cmd.WriteString(putappendArgs.Op)
	cmd.WriteString(common.OPSeparator)
	cmd.WriteString(putappendArgs.Key)
	cmd.WriteString(common.KeyValueSeparator)
	cmd.WriteString(putappendArgs.Value)
	args := &common.PutAppendArgs{
		Op: common.Op{
			ContextId: strconv.FormatInt(nrand(), 36) + CONTEXTSEPARATOR + strconv.FormatInt(nrand(), 36),
			Cmd:       cmd.String(),
		},
	}
	// var prevLdr int = ck.lastLeader
	// DPrintf("begin args: %v", args)
	for {
		if ck.lastLeader < 0 {
			for i := range ck.servers {
				reply := &common.PutAppendReply{}
				ok := ck.servers[i].Call("RaftKV.PutAppend", args, reply)
				if !ok || reply.WrongLeader {
					common.DPrintf("(?)Rpc append failed: reply: %v, ctx: %v", reply, args.Op.ContextId)
					// DPrintf("noleader %v, %v, %v", ok, reply.WrongLeader, args)
					continue
				}
				ck.lastLeader = i

				if reply.Err == common.OK {
					if args.Idx > 0 {
						common.DPrintf("Success append: contextID: %s", args.Op.ContextId)
						return
					}
					if args.Idx < reply.Idx {
						args.Idx = reply.Idx
					}
					args.Tries = 1
				} else if args.Idx > 0 && reply.Err == common.ErrNoKey {
					common.DPrintf("failed append: contextID: %s", args.Op.ContextId)
					args.Tries = 0
					args.Idx = 0
				}

				break
			}
		} else {
			reply := &common.PutAppendReply{}
			ok := ck.servers[ck.lastLeader].Call("RaftKV.PutAppend", args, reply)
			if !ok || reply.WrongLeader {
				// DPrintf("noleader2  %v, %v, %v", ok, reply.WrongLeader, args)
				// prevLdr = ck.lastLeader
				common.DPrintf("(?)Rpc append failed: reply: %v, ctx: %v", reply, args.Op.ContextId)
				if reply.WrongLeader {
					ck.lastLeader = -1
				}
				continue
			}
			if reply.Err == common.OK {
				if args.Idx > 0 {
					common.DPrintf("Success append: contextID: %s", args.Op.ContextId)
					return
				}
				if args.Idx < reply.Idx {
					args.Idx = reply.Idx
				}
				args.Tries = 1
			} else if args.Idx > 0 && reply.Err == common.ErrNoKey {
				common.DPrintf("failed append: contextID: %s", args.Op.ContextId)
				args.Tries = 0
				args.Idx = 0
			}

		}
	}

}

// func (ck *Clerk) Put(key string, value string) {
// 	ck.PutAppend(key, value, "Put")
// }
// func (ck *Clerk) Append(key string, value string) {
// 	ck.PutAppend(key, value, "Append")
// }
