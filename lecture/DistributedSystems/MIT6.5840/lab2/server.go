package kvsrv

import (
	"fmt"
	"log"
	"sync"
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type Snapshot struct {
	sequence int
	val      string
}

type KVServer struct {
	mu   sync.Mutex
	data map[string]string
	log  map[int]Snapshot
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	DPrintf("Receive get request from %d, key: %s", args.RId.ClientId, args.Key)
	kv.mu.Lock()
	defer kv.mu.Unlock()
	reply.Value = ""
	if val, ok := kv.data[args.Key]; ok {
		reply.Value = val
	}
}

func (kv *KVServer) Put(args *PutAppendArgs, reply *PutAppendReply) {
	DPrintf("Receive put request from %d:%d, key: %s", args.RId.ClientId, args.RId.Sequence, args.Key)
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if snap, ok := kv.log[args.RId.ClientId]; ok {
		if snap.sequence == args.RId.Sequence {
			// duplicate request, ensure is executed exactly once
			return
		}
	}
	kv.data[args.Key] = args.Value
	kv.log[args.RId.ClientId] = Snapshot{
		sequence: args.RId.Sequence,
	}
}

func (kv *KVServer) Append(args *PutAppendArgs, reply *PutAppendReply) {
	DPrintf("Receive append request from %d:%d, key: %s", args.RId.ClientId, args.RId.Sequence, args.Key)
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if snap, ok := kv.log[args.RId.ClientId]; ok {
		if snap.sequence == args.RId.Sequence {
			// duplicate request, ensure is executed exactly once
			reply.Value = snap.val
			return
		}
	}
	reply.Value = ""
	if val, ok := kv.data[args.Key]; ok {
		reply.Value = val
	}
	kv.data[args.Key] = fmt.Sprintf("%s%s", reply.Value, args.Value)
	kv.log[args.RId.ClientId] = Snapshot{
		sequence: args.RId.Sequence,
		val:      reply.Value,
	}
}

func StartKVServer() *KVServer {
	kv := new(KVServer)
	kv.data = make(map[string]string)
	kv.log = make(map[int]Snapshot)
	return kv
}
