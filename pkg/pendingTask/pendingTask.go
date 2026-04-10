package pendingTask

import (
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

const shardCount = 64

type shard struct {
	mu sync.Mutex
	m  map[int64]*pendingState
}

type pendingState struct {
	pending int32
	failed  int32
}

type PendingManager struct {
	shards   [shardCount]shard
	doneFunc DoneFunc
	failFunc FailFunc
}
type DoneFunc func(msgID int64)
type FailFunc func(msgID int64, failedCount int32)

var DefaultPendingManager = NewPendingManager()

func NewPendingManager() *PendingManager {
	pm := &PendingManager{}
	for i := 0; i < shardCount; i++ {
		pm.shards[i].m = make(map[int64]*pendingState)
	}
	pm.doneFunc = func(msgID int64) {
		zap.L().Warn("PendingManager doneFunc not set, msgID:", zap.Int64("msgID", msgID))
	}
	pm.failFunc = func(msgID int64, failedCount int32) {
		zap.L().Warn("PendingManager failFunc not set",
			zap.Int64("msgID", msgID),
			zap.Int32("failed_count", failedCount))
	}
	return pm
}

func (pm *PendingManager) SetDoneFunc(f DoneFunc) {
	pm.doneFunc = f
}

func (pm *PendingManager) SetFailFunc(f FailFunc) {
	pm.failFunc = f
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

	s.m[msgID] = &pendingState{pending: count}
}

func (pm *PendingManager) Done(msgID int64) {
	pm.DoneN(msgID, 1)
}

func (pm *PendingManager) DoneN(msgID int64, n int32) {
	s := pm.getShard(msgID)
	state, ok := s.m[msgID]
	if !ok {
		return
	}

	newPending := atomic.AddInt32(&state.pending, -n)
	if newPending > 0 {
		return
	}

	s.mu.Lock()
	delete(s.m, msgID)
	s.mu.Unlock()

	failedCount := atomic.LoadInt32(&state.failed)
	if failedCount > 0 {
		pm.failFunc(msgID, failedCount)
		return
	}
	pm.doneFunc(msgID)
}

func (pm *PendingManager) Fail(msgID int64) {
	pm.FailN(msgID, 1)
}

func (pm *PendingManager) FailN(msgID int64, n int32) {
	s := pm.getShard(msgID)
	state, ok := s.m[msgID]
	if !ok {
		return
	}

	atomic.AddInt32(&state.failed, n)
	newPending := atomic.AddInt32(&state.pending, -n)
	if newPending > 0 {
		return
	}

	s.mu.Lock()
	delete(s.m, msgID)
	s.mu.Unlock()

	pm.failFunc(msgID, atomic.LoadInt32(&state.failed))
}

func (pm *PendingManager) Delete(msgID int64) {
	s := pm.getShard(msgID)
	s.mu.Lock()
	delete(s.m, msgID)
	s.mu.Unlock()
}
