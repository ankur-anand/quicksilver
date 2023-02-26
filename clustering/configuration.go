package clustering

// RaftConfiguration groups all the configuration object for current member(node) related for raft protocol cluster
// initialization.
type RaftConfiguration struct {
	// RaftVolumeDir is the path for raft cluster to store and load configuration
	// and log storage.
	RaftVolumeDir string `json:"raft_volume_dir,omitempty"`
	// RaftApplyTimeoutMs is the timeout duration for quorum inside raft cluster
	// default is 1 sec
	RaftApplyTimeoutMs int64 `json:"raft_apply_timeout_ms,omitempty"`
	// RaftSnapshotIntervalSec controls how often raft should perform a snapshot.
	// default is 20 sec
	RaftSnapshotIntervalSec int32 `json:"raft_snapshot_interval_sec,omitempty"`
	// SnapshotThreshold controls how many outstanding logs there must be before
	// we perform a snapshot. This is to prevent excessive snapshots when we can
	// just replay a small set of logs.
	SnapshotThreshold uint64 `json:"snapshot_threshold,omitempty"` // currently used in default 8192,
	MinLogLevel       LogLevel
}

// MembershipConfiguration groups all the configuration object for current member(node) to initialize the Gossip protocol
// communication.
type MembershipConfiguration struct {
	NodeName      string // should be unique.
	Tags          map[string]string
	AdvertiseAddr string
	AdvertisePort int
	SnapshotPath  string
	MinLogLevel   LogLevel
}
