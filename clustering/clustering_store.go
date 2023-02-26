package clustering

import (
	"sync"
	"sync/atomic"
)

const (
	isLeader    = int32(1)
	isNotLeader = int32(0)
)

// inMemoryStore provides in memory storage for clustering .
type inMemoryStore struct {
	isLeader int32
	// Notifies about the Event to UserSpace.
	en EventNotifier

	l       sync.RWMutex
	members map[string]MemberInformation
	leader  MemberInformation
}

func newInMemoryStore(en EventNotifier) *inMemoryStore {
	return &inMemoryStore{
		l:       sync.RWMutex{},
		members: make(map[string]MemberInformation),
		leader:  MemberInformation{},
		en:      en,
	}
}

// OnEvent callback's the provider with the named event and it's payload.
func (m *inMemoryStore) OnEvent(eventName string, payload []byte) {
	m.en.OnEvent(eventName, payload)
}

// MembersInfo returns all the current active and healthy members(nodes) of the cluster.
func (m *inMemoryStore) MembersInfo() map[string]MemberInformation {
	m.l.RLock()
	defer m.l.RUnlock()
	members := make(map[string]MemberInformation)
	for k, v := range m.members {
		members[k] = v.Clone()
	}
	return members
}

// LeaderInfo provides the information about the member(node) which is currently acting as leader inside the cluster.
func (m *inMemoryStore) LeaderInfo() MemberInformation {
	m.l.RLock()
	defer m.l.RUnlock()
	return m.leader.Clone()
}

// IsCurrentlyLeader reports whether the given member(node) is in leader state inside the cluster.
func (m *inMemoryStore) IsCurrentlyLeader() bool {
	return atomic.LoadInt32(&m.isLeader) == isLeader
}

func (m *inMemoryStore) OnChangeEvent(event MemberEvent, info MemberInformation) {
	switch event {
	case MemberEventJoin:
		m.join(info)
	case MemberEventLeave:
		m.leave(info)
	case MemberEventLeaderChange:
		m.changeLeader(info)
	}

	// pass this information for metrics
	m.en.OnChangeEvent(event, info)
}

func (m *inMemoryStore) join(mi MemberInformation) {
	m.l.Lock()
	defer m.l.Unlock()
	// clone the tags map for concurrent safe ops in other part.
	m.members[mi.NodeName] = mi.Clone()
}

func (m *inMemoryStore) leave(mi MemberInformation) {
	m.l.Lock()
	defer m.l.Unlock()
	delete(m.members, mi.NodeName)
}

func (m *inMemoryStore) changeLeader(mi MemberInformation) {
	m.l.Lock()
	defer m.l.Unlock()
	m.leader = mi.Clone()
}

func (m *inMemoryStore) setLeaderFlag(flag bool) {
	if flag {
		atomic.StoreInt32(&m.isLeader, isLeader)
	} else {
		atomic.StoreInt32(&m.isLeader, isNotLeader)
	}
}
