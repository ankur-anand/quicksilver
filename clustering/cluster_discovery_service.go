package clustering

import (
	"fmt"
	"go.uber.org/zap"
)

// MemberEvent denotes the kind of event that has happened inside the cluster.
type MemberEvent uint

const (
	// MemberEventJoin denotes the member(node) have Joined the membership cluster.
	MemberEventJoin MemberEvent = 1
	// MemberEventLeave denotes the member(node) have Left the membership cluster.
	MemberEventLeave MemberEvent = 2
	// MemberEventLeaderChange denotes there is change in the Leadership inside the cluster.
	MemberEventLeaderChange MemberEvent = 3
)

func (m MemberEvent) String() string {
	switch m {
	case MemberEventJoin:
		return "member-join"
	case MemberEventLeave:
		return "member-leave"
	case MemberEventLeaderChange:
		return "member-leader-change"
	default:
		panic(fmt.Sprintf("unknown event type: %d", m))
	}
}

// MemberInformation stores information object for one (member) node in cluster.
type MemberInformation struct {
	NodeName string
	Tags     map[string]string
}

// Clone Deep clone's the MemberInfo Object.
func (mi MemberInformation) Clone() MemberInformation {
	m := make(map[string]string)
	for k, v := range mi.Tags {
		m[k] = v
	}

	return MemberInformation{
		NodeName: mi.NodeName,
		Tags:     mi.Tags,
	}
}

// ClusterDiscoverService is an interface that groups the ClusterDiscover, UserEvent and ClusterInformer.
type ClusterDiscoverService interface {
	ClusterDiscover
	UserEvent
	ClusterInformer
}

// ClusterDiscover is an interface that wraps the basic OnChangeEvent Method.
// OnChangeEvent notifies the provider when there is a change in the cluster dynamics.
type ClusterDiscover interface {
	OnChangeEvent(event MemberEvent, information MemberInformation)
}

// UserEvent is an interface that wraps the basic SendEvent and OnEvent Method.
// SendEvent is used for generating a custom user type event inside the cluster.
type UserEvent interface {
	SendEvent(name string, payload []byte) error
}

// ClusterInformer is an interface that wraps the LeaderInfo() Method.
// MembersInfo returns all the current active and healthy members(nodes) of the cluster.
// LeaderInfo provides the information about the member(node) which is currently acting as leader inside the cluster.
// IsCurrentlyLeader reports whether the given member(node) is in leader state inside the cluster.
type ClusterInformer interface {
	MembersInfo() map[string]MemberInformation
	LeaderInfo() MemberInformation
	IsCurrentlyLeader() bool
}

// ClusterBuilder is an interface that wraps the Join Methods.
// Join joins an existing cluster.
type ClusterBuilder interface {
	Join([]string) error
}

// EventNotifier is an interface that groups ClusterDiscover Interface and OnEvent Method.
// EventNotifier is used by the default ClusterDiscoverService implementation to send notification about cluster change.
type EventNotifier interface {
	ClusterDiscover
	OnEvent(eventName string, payload []byte)
}

type ClusterDiscoveryConfiguration struct {
	MembershipConfiguration MembershipConfiguration
	RaftConfiguration       RaftConfiguration
}

// ClusterDiscovery creates an raft and gossip based ClusterDiscoverService for consumers.
type ClusterDiscovery struct {
	// UserInputs.
	ClusterDiscoveryConfiguration ClusterDiscoveryConfiguration
	logger                        *zap.Logger
	en                            EventNotifier

	// gossip.
	memShip membership

	// close
	shutdownCh chan struct{}

	*inMemoryStore
}

func (cd *ClusterDiscovery) Join(existing []string) error {
	_, err := cd.memShip.Join(existing, true)
	return err
}

// SendEvent generates the provided named event inside the cluster with the provided payload.
// IMP: Payload size is limited: inMemoryStore gossips via UDP, so the payload must fit within a single UDP packet.
func (cd *ClusterDiscovery) SendEvent(name string, payload []byte) error {
	payloadSizeBeforeEncoding := len(name) + len(payload)
	if payloadSizeBeforeEncoding > 512 {
		return fmt.Errorf(
			"user event exceeds configured limit of %d bytes before encoding",
			512,
		)
	}
	return cd.memShip.Serf.UserEvent(name, payload, false)
}

func NewClusterDiscovery(logger *zap.Logger, cdc ClusterDiscoveryConfiguration, en EventNotifier) (*ClusterDiscovery, error) {
	discovery := ClusterDiscovery{
		ClusterDiscoveryConfiguration: cdc,
		logger:                        logger,
		en:                            en,
		shutdownCh:                    make(chan struct{}),
		inMemoryStore:                 newInMemoryStore(en),
	}

	// create membership first.
	memship, err := newMembership(cdc.MembershipConfiguration, logger, discovery.inMemoryStore)
	if err != nil {
		return nil, err
	}
	discovery.memShip = memship

	go func() {
		err := memship.eventHandler(discovery.shutdownCh)
		if err != nil {
			panic(err)
		}
	}()

	// create raft instance.
	// wraps the provided en event notifier with raft event notifier to dynamically add and remove
	// node from the cluster.

	return &discovery, nil
}
