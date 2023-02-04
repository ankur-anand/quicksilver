package clustering_test

import (
	"github.com/ankur-anand/quicksilver/clustering"
	"github.com/hashicorp/serf/serf"
	"go.uber.org/zap/zaptest"
	"net"
	"strconv"
	"testing"
)

func TestSetupSerf(t *testing.T) {
	mc1 := _setupMemberConfig(t)
	l := zaptest.NewLogger(t)
	ec1 := make(chan serf.Event, 10)
	serf1, err := clustering.NewMembership(ec1, mc1, l)
	if err != nil {
		t.Fatal(err)
	}
	go eventHandler(serf1.Serf, ec1)
	mc2 := _setupMemberConfig(t)
	p := strconv.FormatInt(int64(mc1.AdvertisePort), 10)
	addr := net.JoinHostPort("127.0.0.1", p)

	ec2 := make(chan serf.Event, 5)
	serf2, err := clustering.NewMembership(ec2, mc2, l)
	if err != nil {
		t.Fatal(err)
	}
	go eventHandler(serf2.Serf, ec2)
	_, err = serf2.Join([]string{addr}, true)
	if err != nil {
		t.Fatal(err)
	}

	err = serf2.UserEvent("raft-add", []byte("new raft member added"), false)
	if err != nil {
		t.Fatal(err)
	}
	err = serf2.Leave()
	if err != nil {
		t.Fatal(err)
	}
	err = serf2.Shutdown()
	if err != nil {
		t.Fatal(err)
	}
}

func _setupMemberConfig(t *testing.T) clustering.MembershipConfiguration {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	err = listener.Close()
	if err != nil {
		t.Fatal(err)
	}
	p := strconv.FormatInt(int64(port), 10)
	addr := net.JoinHostPort("127.0.0.1", p)
	tmpDir := t.TempDir()
	mc := clustering.MembershipConfiguration{
		NodeName:      addr,
		AdvertiseAddr: "127.0.0.1",
		AdvertisePort: port,
		SnapshotPath:  tmpDir,
		Tags: map[string]string{
			"quicksilver-raft-addr":        net.JoinHostPort("127.0.0.1", "8080"),
			"quicksilver-raft-is_suffrage": "true",
		},
	}
	return mc
}

func eventHandler(serfIns *serf.Serf, events chan serf.Event) error {
	for e := range events {
		switch e.EventType() {
		case serf.EventMemberJoin:
			for _, member := range e.(serf.MemberEvent).Members {
				if isLocal(serfIns, member) {
					continue
				}
			}
		case serf.EventMemberLeave, serf.EventMemberFailed, serf.EventMemberReap:
			for _, member := range e.(serf.MemberEvent).Members {
				if isLocal(serfIns, member) {
					continue
				}
			}
		case serf.EventUser:
			event := e.(serf.UserEvent)
			clonePayload := make([]byte, len(event.Payload))
			copy(clonePayload, event.Payload)
		}
	}
	return nil
}

func isLocal(serfIns *serf.Serf, member serf.Member) bool {
	return serfIns.LocalMember().Name == member.Name
}
