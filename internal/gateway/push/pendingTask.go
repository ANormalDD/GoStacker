package push

import (
	"GoStacker/pkg/db/redis"
	"fmt"
	"sync"
	"sync/atomic"
)

const shardCount = 64 // 可根据并发量调整

type shard struct {
	mu sync.Mutex
	m  map[int64]*int32
}

type PendingManager struct {
	shards [shardCount]shard
}

var defaultPendingManager = NewPendingManager()

func NewPendingManager() *PendingManager {
	pm := &PendingManager{}
	for i := 0; i < shardCount; i++ {
		pm.shards[i].m = make(map[int64]*int32)
	}
	return pm
}

func (pm *PendingManager) getShard(msgID int64) *shard {
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

	ptr := s.m[msgID]

	newVal := atomic.AddInt32(ptr, -1)
	if newVal <= 0 {
		s.mu.Lock()
		delete(s.m, msgID)
		s.mu.Unlock()
		redis.InsertAckCache(fmt.Sprintf("%d", msgID))
	}
}

func (pm *PendingManager) Delete(msgID int64) {
	s := pm.getShard(msgID)
	s.mu.Lock()
	delete(s.m, msgID)
	s.mu.Unlock()
}
