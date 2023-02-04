package clustering_test

import (
	"github.com/ankur-anand/quicksilver/clustering"
	"github.com/hashicorp/raft"
	"net"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestRaftStream_Request(t *testing.T) {
	t.Parallel()

	getRaftTransport := func() *raft.NetworkTransport {
		// create a stream.
		raftLis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Errorf("errored getting the listener")
			t.SkipNow()
		}

		port := raftLis.Addr().(*net.TCPAddr).Port
		p := strconv.FormatInt(int64(port), 10)
		advA, err := clustering.GetAdvertiseAddr(net.JoinHostPort("127.0.0.1", p))
		if err != nil {
			t.Errorf("errored getting the listener")
			t.SkipNow()
		}

		streamLayer1 := clustering.NewRaftStreamLayer(raftLis, advA, nil, nil)
		return raft.NewNetworkTransport(streamLayer1, 10, 10*time.Second, os.Stderr)
	}

	// trans1 is consumer
	trans1 := getRaftTransport()
	defer trans1.Close()
	rpcCh := trans1.Consumer()
	// Make the RPC request
	args := raft.AppendEntriesRequest{
		Term:         10,
		Leader:       []byte("quicksilver"),
		PrevLogEntry: 100,
		PrevLogTerm:  4,
		Entries: []*raft.Log{
			{
				Index: 101,
				Term:  4,
				Type:  raft.LogNoop,
			},
		},
		LeaderCommitIndex: 90,
	}
	resp := raft.AppendEntriesResponse{
		Term:    4,
		LastLog: 90,
		Success: true,
	}
	// Listen for a request
	go func() {
		for {
			select {
			case rpc := <-rpcCh:
				// Verify the command
				req := rpc.Command.(*raft.AppendEntriesRequest)
				if !reflect.DeepEqual(req, &args) {
					t.Errorf("command mismatch: %#v %#v", *req, args)
				}
				rpc.Respond(&resp, nil)

			case <-time.After(10 * time.Second):
				return
			}
		}
	}()

	trans2 := getRaftTransport()

	defer trans2.Close()
	var i int
	for i = 0; i < 2; i++ {
		var out raft.AppendEntriesResponse
		if err := trans2.AppendEntries("id1", trans1.LocalAddr(), &args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Verify the response
		if !reflect.DeepEqual(resp, out) {
			t.Fatalf("command mismatch: %#v %#v", resp, out)
		}
	}

}
