package clustering

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/hashicorp/raft"
	"net"
	"time"
)

const RaftRPC = 1

var _ raft.StreamLayer = (*RaftStreamLayer)(nil)

// RaftStreamLayer Provides implementation for Raft's Stream Layer interface
type RaftStreamLayer struct {
	advertise net.Addr
	ln        net.Listener
	// Enable encrypted communication between servers with TLS
	// serverTLS accept incoming TLSConnection
	serverTLS *tls.Config
	// peerTLS create outgoing TLSConnection
	peerTLS *tls.Config
}

// NewRaftStreamLayer returns a new initialized Stream Layer for network transport.
func NewRaftStreamLayer(ln net.Listener, advertiseAddr net.Addr, serverTLS, peerTLS *tls.Config) *RaftStreamLayer {
	return &RaftStreamLayer{
		advertise: advertiseAddr,
		ln:        ln,
		serverTLS: serverTLS,
		peerTLS:   peerTLS,
	}
}

func (s *RaftStreamLayer) Accept() (net.Conn, error) {
	conn, err := s.ln.Accept()
	if err != nil {
		return nil, err
	}

	b := make([]byte, 1)
	_, err = conn.Read(b)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal([]byte{byte(RaftRPC)}, b) {
		return nil, fmt.Errorf("not a raft rpc")
	}

	if s.serverTLS != nil {
		return tls.Server(conn, s.serverTLS), nil
	}
	return conn, nil
}

func (s *RaftStreamLayer) Close() error {
	return s.ln.Close()
}

func (s *RaftStreamLayer) Addr() net.Addr {
	// Use an advertise addr if provided
	if s.advertise != nil {
		return s.advertise
	}
	return s.ln.Addr()
}

// Dial makes outgoing connection to other servers in the raft cluster.
func (s *RaftStreamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	var conn, err = dialer.Dial("tcp", string(address))
	if err != nil {
		return nil, err
	}

	// raft identification to mux
	_, err = conn.Write([]byte{byte(RaftRPC)})
	if err != nil {
		return nil, err
	}

	// upgrade connection to TLS
	if s.peerTLS != nil {
		conn = tls.Client(conn, s.peerTLS)
	}
	return conn, err
}

// GetAdvertiseAddr returns and verify that we have a usable advertise address
func GetAdvertiseAddr(advAddr string) (net.Addr, error) {
	advertiseAddr, err := net.ResolveTCPAddr("tcp", advAddr)
	if err != nil {
		return nil, err
	}

	if advertiseAddr.IP == nil || advertiseAddr.IP.IsUnspecified() {
		return nil, fmt.Errorf("address %s is not advertisable", advAddr)
	}
	return advertiseAddr, nil
}
