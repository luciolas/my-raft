package clientdto

type ClientGetArg struct {
	Key string `json:"key,omitempty" query:"key"`
}

type ClientGetReply struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type ClientPutAppendArg struct {
	Key   string `json:"key,omitempty"`
	Op    string `json:"op,omitempty"`
	Value string `json:"value,omitempty"`
}

type ClientPutAppendReply struct {
}
