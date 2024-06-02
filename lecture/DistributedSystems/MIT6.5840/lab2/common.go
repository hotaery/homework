package kvsrv

type RequestId struct {
	ClientId int
	Sequence int
}

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	RId   RequestId
}

type PutAppendReply struct {
	Value string
}

type GetArgs struct {
	Key string
	RId RequestId
}

type GetReply struct {
	Value string
}
