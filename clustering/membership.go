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

// MembershipConfiguration Configuration stores gossip configuration object for current node.
type MembershipConfiguration struct {
	NodeName      string // should be unique.
	Tags          map[string]string
	AdvertiseAddr string
	AdvertisePort int
	SnapshotPath  string
	MinLogLevel   LogLevel
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

// Membership provides the gossip membership inside the cluster.
// It help's discover the nodes and it's metadata dynamically and quickly inside the cluster.
type Membership struct {
	*serf.Serf
}

// NewMembership is used to setup and initialize a gossip Membership
func NewMembership(eventsCh chan serf.Event, c MembershipConfiguration, logger *zap.Logger) (Membership, error) {
	conf := defaultConfig()
	conf.Init()
	conf.NodeName = c.NodeName
	conf.SnapshotPath = filepath.Join(c.SnapshotPath, serfSnapshot)
	if err := ensurePath(conf.SnapshotPath, false); err != nil {
		return Membership{}, err
	}

	conf.Tags = c.Tags
	conf.EventCh = eventsCh
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
	return Membership{Serf: s}, err
}

type lwr struct {
	lFunc func(msg string, fields ...zap.Field)
}

func (l *lwr) Write(p []byte) (int, error) {
	p = bytes.TrimSpace(p)
	l.lFunc(string(p))
	return len(p), nil
}
