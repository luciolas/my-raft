package data

import "sync/atomic"

type At32 struct {
	val int32
}

func (at *At32) Set(v int32) {
	atomic.StoreInt32(&at.val, v)
}

func (at *At32) Load() int32 {
	return atomic.LoadInt32(&at.val)
}

func (at *At32) Add(v int32) int32 {
	return atomic.AddInt32(&at.val, v)
}

type AtBool struct {
	val uint32
}

func (at *AtBool) Set(v bool) {
	var in uint32
	if v {
		in = 1
	}
	atomic.StoreUint32(&at.val, in)
}

func (at *AtBool) Load() bool {
	return atomic.LoadUint32(&at.val) != 0
}

type AtU64 struct {
	val uint64
}

func (at *AtU64) Set(v uint64) {
	atomic.StoreUint64(&at.val, v)
}

func (at *AtU64) Load() uint64 {
	return atomic.LoadUint64(&at.val)
}

func (at *AtU64) Add(v uint64) uint64 {
	return atomic.AddUint64(&at.val, v)
}
