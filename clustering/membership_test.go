package clustering

import (
	"go.uber.org/zap/zaptest"
	"net"
	"strconv"
	"testing"
	"time"
)

type mockedEventNotifier struct {
	counter chan int
	eventer chan int
}

func (m mockedEventNotifier) OnChangeEvent(event MemberEvent, information MemberInformation) {
	m.counter <- 1
}

func (m mockedEventNotifier) OnEvent(eventName string, payload []byte) {
	m.eventer <- 1
}

func newMockedEN() mockedEventNotifier {
	return mockedEventNotifier{eventer: make(chan int, 10), counter: make(chan int, 10)}
}

func TestSetupSerf(t *testing.T) {
	mc1 := _setupMemberConfig(t)
	l := zaptest.NewLogger(t)
	shutdownCh := make(chan struct{})

	mcen1 := newMockedEN()
	en1 := newInMemoryStore(mcen1)

	serf1, err := newMembership(mc1, l, en1)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		err := serf1.eventHandler(shutdownCh)
		if err != nil {
			panic(err)
		}
	}()

	mc2 := _setupMemberConfig(t)
	p := strconv.FormatInt(int64(mc1.AdvertisePort), 10)
	addr := net.JoinHostPort("127.0.0.1", p)

	mcen2 := newMockedEN()
	en2 := newInMemoryStore(mcen2)
	serf2, err := newMembership(mc2, l, en2)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		err := serf2.eventHandler(shutdownCh)
		if err != nil {
			panic(err)
		}
	}()
	_, err = serf2.Join([]string{addr}, true)
	if err != nil {
		t.Fatal(err)
	}

	err = serf2.sendEvent("raft-add", []byte("new raft member added"))
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

	validateEvents := func(count chan int, want int) {
		select {
		case <-time.After(5 * time.Second):
			t.Errorf("errored while waiting for chan value.")
		case c := <-count:
			if c != want {
				t.Errorf("expected channel value didn't matched want %d got %d", want, c)
			}
		}
	}

	for _, ch := range []chan int{mcen1.counter, mcen2.counter, mcen1.eventer, mcen2.eventer} {
		validateEvents(ch, 1)
	}
}

func _setupMemberConfig(t *testing.T) MembershipConfiguration {
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
	mc := MembershipConfiguration{
		NodeName:      addr,
		AdvertiseAddr: "127.0.0.1",
		AdvertisePort: port,
		SnapshotPath:  tmpDir,
		Tags: map[string]string{
			"quicksilver-test-addr": net.JoinHostPort("127.0.0.1", "8080"),
		},
	}
	return mc
}
