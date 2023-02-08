package clustering

import (
	"bytes"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/serf/serf"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"time"
)

const (
	serfSnapshot = "serf/config.snapshot"
)

// LogLevel defines log level for clustering.
type LogLevel int

const (
	// logLevelUnset prevents the default value for go type system becoming the log level.
	logLevelUnset LogLevel = iota
	LogLevelDebug
	LoglevelInfo
	LogLevelWarn
	LogLevelError
)

func (l LogLevel) ToString(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "DEBUG"
	case LoglevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	}
	// default return info level logs.
	return "INFO"
}

// ensurePath is used to make sure a path exists
func ensurePath(path string, dir bool) error {
	if !dir {
		path = filepath.Dir(path)
	}
	return os.MkdirAll(path, 0755)
}

// DefaultConfig returns a quicksilver-flavored Serf default configuration,
// suitable as a basis for a LAN, WAN, segment, or area.
func defaultConfig() *serf.Config {
	base := serf.DefaultConfig()

	// This effectively disables the annoying queue depth warnings.
	base.QueueDepthWarning = 1000000

	// This enables dynamic sizing of the message queue depth based on the
	// cluster size.
	base.MinQueueDepth = 4096

	// This gives leaves some time to propagate through the cluster before
	// we shut down. The value was chosen to be reasonably short, but to
	// allow a leave to get to over 99.99% of the cluster with 100k nodes
	// (using https://www.serf.io/docs/internals/simulator.html).
	base.LeavePropagateDelay = 3 * time.Second

	return base
}

// membership provides the gossip membership inside the cluster.
// It help's discover the nodes and it's metadata dynamically and quickly inside the cluster.
type membership struct {
	*serf.Serf
	eventsCh chan serf.Event
	en       EventNotifier
}

// SendEvent generates the provided named event inside the cluster with the provided payload.
// IMP: Payload size is limited: inMemoryStore gossips via UDP, so the payload must fit within a single UDP packet.
func (m *membership) sendEvent(name string, payload []byte) error {
	return m.Serf.UserEvent(name, payload, false)
}

func (m *membership) Close() {

}

// EventHandler handles events operation for membership cluster.
func (m *membership) eventHandler(shutdownCh <-chan struct{}) error {
	for {
		select {
		case <-shutdownCh:
			return nil
		case event := <-m.eventsCh:
			switch event.EventType() {
			case serf.EventMemberJoin:
				for _, member := range event.(serf.MemberEvent).Members {
					if m.isLocal(member) {
						continue
					}
					mi := MemberInformation{
						NodeName: member.Name,
						Tags:     member.Tags,
					}
					m.en.OnChangeEvent(MemberEventJoin, mi)
				}
			case serf.EventMemberLeave, serf.EventMemberFailed, serf.EventMemberReap:
				for _, member := range event.(serf.MemberEvent).Members {
					if m.isLocal(member) {
						continue
					}
					mi := MemberInformation{
						NodeName: member.Name,
						Tags:     member.Tags,
					}
					m.en.OnChangeEvent(MemberEventLeave, mi)
				}
			case serf.EventUser:
				ue := event.(serf.UserEvent)
				m.en.OnEvent(ue.Name, ue.Payload)

			}
		}
	}
}

func (m *membership) isLocal(member serf.Member) bool {
	return m.Serf.LocalMember().Name == member.Name
}

// newMembership is used to setup and initialize a gossip membership
func newMembership(c MembershipConfiguration, logger *zap.Logger, en EventNotifier) (membership, error) {
	// serfEventChSize is the size of the buffered channel to get Serf
	// events. If this is exhausted we will block Serf and Memberlist.
	serfEventChSize := 2048
	serfEventsCh := make(chan serf.Event, serfEventChSize)
	conf := defaultConfig()
	conf.Init()
	conf.NodeName = c.NodeName
	conf.SnapshotPath = filepath.Join(c.SnapshotPath, serfSnapshot)
	if err := ensurePath(conf.SnapshotPath, false); err != nil {
		return membership{}, err
	}

	conf.Tags = c.Tags
	conf.EventCh = serfEventsCh
	// TODO: Support Key Encryption for serf events.

	// This is the best effort to convert the standard log,
	// to the zap format.
	// from the serf and gossip library.
	logger = logger.With()
	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel(c.MinLogLevel.ToString(c.MinLogLevel)),
		Writer:   &lwr{lFunc: logger.Info},
	}

	stdLog := zap.NewStdLog(logger)
	stdLog.SetOutput(filter)

	conf.MemberlistConfig.Logger = stdLog
	conf.MemberlistConfig.AdvertiseAddr = c.AdvertiseAddr
	conf.MemberlistConfig.AdvertisePort = c.AdvertisePort
	conf.MemberlistConfig.BindPort = c.AdvertisePort
	conf.Logger = stdLog
	s, err := serf.Create(conf)
	return membership{Serf: s, eventsCh: serfEventsCh, en: en}, err
}

type lwr struct {
	lFunc func(msg string, fields ...zap.Field)
}

func (l *lwr) Write(p []byte) (int, error) {
	p = bytes.TrimSpace(p)
	l.lFunc(string(p))
	return len(p), nil
}
