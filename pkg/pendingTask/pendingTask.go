package pendingTask

import (
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

const shardCount = 64

type shard struct {
	mu sync.Mutex
	m  map[int64]*int32
}

type PendingManager struct {
	shards   [shardCount]shard
	doneFunc DoneFunc
}
type DoneFunc func(msgID int64)

var DefaultPendingManager = NewPendingManager()

func NewPendingManager() *PendingManager {
	pm := &PendingManager{}
	for i := 0; i < shardCount; i++ {
		pm.shards[i].m = make(map[int64]*int32)
	}
	pm.doneFunc = func(msgID int64) {
		zap.L().Warn("PendingManager doneFunc not set, msgID:", zap.Int64("msgID", msgID))
	}
	return pm
}

func (pm *PendingManager) SetDoneFunc(f DoneFunc) {
	pm.doneFunc = f
}

func (pm *PendingManager) getShard(msgID int64) *shard {
	if msgID < 0 {
		msgID = -msgID
	}
	return &pm.shards[msgID%shardCount]
}

// 初始化一个 msgID 的 pendingCount
func (pm *PendingManager) Init(msgID int64, count int32) {
	s := pm.getShard(msgID)
	s.mu.Lock()
	defer s.mu.Unlock()

	var v int32 = count
	s.m[msgID] = &v
}

func (pm *PendingManager) Done(msgID int64) {
	s := pm.getShard(msgID)

	ptr, ok := s.m[msgID]
	if !ok {
		return
	}
	newVal := atomic.AddInt32(ptr, -1)
	if newVal <= 0 {
		s.mu.Lock()
		delete(s.m, msgID)
		s.mu.Unlock()
		pm.doneFunc(msgID)
	}
}
func (pm *PendingManager) DoneN(msgID int64, n int32) {
	s := pm.getShard(msgID)
	ptr, ok := s.m[msgID]
	if !ok {
		return
	}
	newVal := atomic.AddInt32(ptr, -n)
	if newVal <= 0 {
		s.mu.Lock()
		delete(s.m, msgID)
		s.mu.Unlock()
		pm.doneFunc(msgID)
	}
}

func (pm *PendingManager) Delete(msgID int64) {
	s := pm.getShard(msgID)
	s.mu.Lock()
	delete(s.m, msgID)
	s.mu.Unlock()
}
