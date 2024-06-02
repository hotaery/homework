package kvsrv

import (
	"fmt"
	"labrpc"
)

var globalClientId int = 0

type Clerk struct {
	server       *labrpc.ClientEnd
	clientId     int
	lastSequence int
}

func MakeClerk(server *labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.server = server
	ck.clientId = globalClientId
	ck.lastSequence = 0
	globalClientId++
	return ck
}

func (ck *Clerk) Get(key string) string {
	ok := false
	args := GetArgs{
		Key: key,
		RId: RequestId{
			ClientId: ck.clientId,
		},
	}
	reply := GetReply{}
	for !ok {
		ok = ck.server.Call("KVServer.Get", &args, &reply)
	}
	return reply.Value
}

func (ck *Clerk) PutAppend(key string, value string, op string) string {
	ok := false
	args := PutAppendArgs{
		Key:   key,
		Value: value,
		RId: RequestId{
			ClientId: ck.clientId,
			Sequence: ck.lastSequence,
		},
	}
	reply := PutAppendReply{}
	for !ok {
		ok = ck.server.Call(fmt.Sprintf("KVServer.%s", op), &args, &reply)
	}
	ck.lastSequence++
	return reply.Value
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}

// Append value to key's value and return that value
func (ck *Clerk) Append(key string, value string) string {
	return ck.PutAppend(key, value, "Append")
}
