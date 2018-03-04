package app

import (
	"sync"
	"time"
)

type addrMap struct {
	m  map[string]time.Time
	mu sync.RWMutex
}

func newAddrMap() *addrMap {
	return &addrMap{
		m: make(map[string]time.Time),
	}
}

func (a *addrMap) Get(addr string) (time.Time, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	time, exists := a.m[addr]
	return time, exists
}

// GetOrSet - Возвращает время, если оно установлено на аддресс или ставит его
// Второй параметр bool:
// true - если уже установлено
// false - если не существовало
func (a *addrMap) GetOrSet(addr string, t time.Time) (time.Time, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if t, ext := a.m[addr]; ext {
		return t, true
	}
	a.m[addr] = t
	return t, false
}

func (a *addrMap) Del(addr string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.m, addr)
}
