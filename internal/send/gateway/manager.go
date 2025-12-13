package gateway

import (
	"container/heap"
	"sync"
)

type GatewayItem struct {
	GatewayID string
	LoadRate  float32
	Address   string
	index     int
}

// gatewayHeap implements heap.Interface and holds GatewayItems.
type gatewayHeap []*GatewayItem

func (h gatewayHeap) Len() int           { return len(h) }
func (h gatewayHeap) Less(i, j int) bool { return h[i].LoadRate < h[j].LoadRate }
func (h gatewayHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *gatewayHeap) Push(x interface{}) {
	it := x.(*GatewayItem)
	it.index = len(*h)
	*h = append(*h, it)
}

func (h *gatewayHeap) Pop() interface{} {
	old := *h
	n := len(old)
	if n == 0 {
		return nil
	}
	it := old[n-1]
	it.index = -1
	*h = old[:n-1]
	return it
}

// Manager maintains a min-heap by LoadRate and a map for O(1) lookup.
// It is safe for concurrent use.
type Manager struct {
	mu     sync.RWMutex
	items  gatewayHeap
	lookup map[string]*GatewayItem
}

// NewManager creates and initializes a Manager.
func NewManager() *Manager {
	m := &Manager{
		lookup: make(map[string]*GatewayItem),
	}
	heap.Init(&m.items)
	return m
}

// DefaultManager 是包级的全局单例 Manager，程序中只会存在一个唯一的 manager。
// 直接使用 gateway.DefaultManager 插入/查询/更新/删除。
var DefaultManager = NewManager()

// Insert inserts a new gateway or updates an existing one.
// Returns the stored GatewayItem pointer.
func (m *Manager) Insert(gatewayID, address string, load float32) *GatewayItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	if it, ok := m.lookup[gatewayID]; ok {
		it.Address = address
		it.LoadRate = load
		heap.Fix(&m.items, it.index)
		return it
	}

	it := &GatewayItem{GatewayID: gatewayID, LoadRate: load, Address: address}
	heap.Push(&m.items, it)
	m.lookup[gatewayID] = it
	return it
}

// Peek returns a copy of the heap-top (minimum LoadRate) item without removing it.
// The bool indicates whether an item was returned.
func (m *Manager) Peek() (GatewayItem, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.items) == 0 {
		return GatewayItem{}, false
	}
	top := m.items[0]
	return *top, true
}

// PopMin removes and returns the heap-top (minimum LoadRate) item.
func (m *Manager) PopMin() (GatewayItem, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.items) == 0 {
		return GatewayItem{}, false
	}
	it := heap.Pop(&m.items).(*GatewayItem)
	delete(m.lookup, it.GatewayID)
	return *it, true
}

// UpdateLoadRate updates the LoadRate for a gateway and fixes the heap.
// Returns true if the gateway existed and was updated.
func (m *Manager) UpdateLoadRate(gatewayID string, newLoad float32) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.lookup[gatewayID]
	if !ok {
		return false
	}
	it.LoadRate = newLoad
	heap.Fix(&m.items, it.index)
	return true
}

// Remove deletes a gateway from the manager. Returns true if removed.
func (m *Manager) Remove(gatewayID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.lookup[gatewayID]
	if !ok {
		return false
	}
	heap.Remove(&m.items, it.index)
	delete(m.lookup, gatewayID)
	return true
}

// Len returns number of items in the heap.
func (m *Manager) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// List returns a snapshot copy of all gateway items (not ordered).
func (m *Manager) List() []GatewayItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]GatewayItem, 0, len(m.items))
	for _, it := range m.items {
		out = append(out, *it)
	}
	return out
}
