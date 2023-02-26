package broadcaster_test

import (
	"context"
	"github.com/ankur-anand/quicksilver/broadcaster"
	"github.com/ankur-anand/quicksilver/proto/gen/v1/quicksilverpb"
	"reflect"
	"testing"
	"time"
)

type client struct {
	name     string
	database string
}

func TestBroadcastToSubscriber_Broadcast(t *testing.T) {
	ctx, done := context.WithCancel(context.Background())
	defer done()
	bc := broadcaster.NewBroadcaster(ctx)
	defer bc.Close()
	clients := []client{
		{
			name:     "client1",
			database: "database1",
		},
		{
			name:     "client2",
			database: "database2",
		},
		{
			name:     "client3",
			database: "database3",
		},
	}

	outStreams := make(map[string]<-chan []*quicksilverpb.TransactionLogs)
	for _, c := range clients {
		stream, err := bc.NewClientSubscriptionStream(c.name, c.database)
		if err != nil {
			t.Errorf("error creating clientStream for cl %s", c.name)
		}
		outStreams[c.name] = stream
	}

	// publish message.
	pubMessages := make(map[string][]*quicksilverpb.TransactionLogs)
	heartBeatMSG := &quicksilverpb.TransactionLogs{
		LogKind:              quicksilverpb.TransactionLogs_HEARTBEAT,
		CreatedUnixTimestamp: time.Now().UnixNano(),
		AppliedToDb:          false,
	}

	pubMessages["database1"] = []*quicksilverpb.TransactionLogs{
		{
			LogKind:              quicksilverpb.TransactionLogs_OPERATIONAL,
			SequenceNumber:       1,
			LastSequenceNumber:   0,
			Operation:            quicksilverpb.TransactionLogs_SET,
			Database:             "database1",
			Kv:                   nil,
			CreatedUnixTimestamp: 1,
			AppliedToDb:          false,
			Trace:                nil,
		},
	}

	pubMessages["database2"] = []*quicksilverpb.TransactionLogs{
		{
			LogKind:              quicksilverpb.TransactionLogs_OPERATIONAL,
			SequenceNumber:       2,
			LastSequenceNumber:   1,
			Operation:            quicksilverpb.TransactionLogs_DELETE,
			Database:             "database2",
			Kv:                   nil,
			CreatedUnixTimestamp: 2,
			AppliedToDb:          false,
			Trace:                nil,
		},
	}

	pubMessages["database3"] = []*quicksilverpb.TransactionLogs{
		{
			LogKind:              quicksilverpb.TransactionLogs_OPERATIONAL,
			SequenceNumber:       3,
			LastSequenceNumber:   2,
			Operation:            quicksilverpb.TransactionLogs_SET,
			Database:             "database3",
			Kv:                   nil,
			CreatedUnixTimestamp: 3,
			AppliedToDb:          false,
			Trace:                nil,
		},
		{
			LogKind:              quicksilverpb.TransactionLogs_OPERATIONAL,
			SequenceNumber:       4,
			LastSequenceNumber:   3,
			Operation:            quicksilverpb.TransactionLogs_DELETE,
			Database:             "database3",
			Kv:                   nil,
			CreatedUnixTimestamp: 3,
			AppliedToDb:          false,
			Trace:                nil,
		},
		{
			LogKind:              quicksilverpb.TransactionLogs_OPERATIONAL,
			SequenceNumber:       5,
			LastSequenceNumber:   4,
			Operation:            quicksilverpb.TransactionLogs_DELETE,
			Database:             "database3",
			Kv:                   nil,
			CreatedUnixTimestamp: 4,
			AppliedToDb:          true,
			Trace:                nil,
		},
	}
	pubMessages["unknown"] = []*quicksilverpb.TransactionLogs{
		{
			LogKind:            quicksilverpb.TransactionLogs_OPERATIONAL,
			SequenceNumber:     0,
			LastSequenceNumber: 0,
			Operation:          quicksilverpb.TransactionLogs_DELETE,
			Database:           "",
			Kv:                 nil,
			AppliedToDb:        false,
			Trace:              nil,
		},
	}

	for _, msgs := range pubMessages {
		for _, m := range msgs {
			bc.Broadcast(m)
		}
	}
	// _ = heartBeatMSG
	bc.Broadcast(heartBeatMSG)

	recvMessages := make(map[string][]*quicksilverpb.TransactionLogs)
	hbCounter := make(map[string]int)
	for c, s := range outStreams {
		select {
		case <-time.After(5 * time.Second):
		case logs := <-s:
			for _, l := range logs {
				// if it's an heartbeat msg
				if l.LogKind == quicksilverpb.TransactionLogs_HEARTBEAT {
					hbCounter[c]++
					continue
				}
				_, ok := recvMessages[l.Database]
				if ok {
					recvMessages[l.Database] = append(recvMessages[l.Database], l)
				} else {
					recvMessages[l.Database] = make([]*quicksilverpb.TransactionLogs, 0)
					recvMessages[l.Database] = append(recvMessages[l.Database], l)
				}
			}
		}
	}

	// all the client should receive exactly one heartbeat msg
	for k, v := range hbCounter {
		if v != 1 {
			t.Errorf("client %s received more than 1 heatbeat msg %d", k, v)
		}
	}
	// In Each database the sequence of the message should be same.
	for db, logs := range recvMessages {
		expectedLogs := pubMessages[db]
		for i, v := range logs {
			exLog := expectedLogs[i]
			if !reflect.DeepEqual(exLog, v) {
				t.Errorf("expected log message in order %v got %v", exLog, v)
			}
		}
	}

	_, ok := recvMessages["unknown"]
	if ok {
		t.Errorf("db clientStream unsubscribed shouldn't have recieved message")
	}
}

func TestBroadcastToSubscriber_EvictClient(t *testing.T) {
	ctx, done := context.WithCancel(context.Background())
	defer done()
	bc := broadcaster.NewBroadcaster(ctx)
	defer bc.Close()
	clients := []client{
		{
			name:     "client1",
			database: "database1",
		},
		{
			name:     "client2",
			database: "database2",
		},
	}

	// publish message.
	pubMessages := make(map[string][]*quicksilverpb.TransactionLogs)
	pubMessages["database1"] = []*quicksilverpb.TransactionLogs{
		{
			LogKind:            quicksilverpb.TransactionLogs_OPERATIONAL,
			SequenceNumber:     1,
			LastSequenceNumber: 0,
			Operation:          quicksilverpb.TransactionLogs_SET,
			Database:           "database1",
			Kv:                 nil,
			AppliedToDb:        false,
			Trace:              nil,
		},
	}

	pubMessages["database2"] = []*quicksilverpb.TransactionLogs{
		{
			LogKind:            quicksilverpb.TransactionLogs_OPERATIONAL,
			SequenceNumber:     2,
			LastSequenceNumber: 1,
			Operation:          quicksilverpb.TransactionLogs_DELETE,
			Database:           "database2",
			Kv:                 nil,
			AppliedToDb:        false,
			Trace:              nil,
		},
	}

	outStreams := make(map[string]<-chan []*quicksilverpb.TransactionLogs)
	for _, c := range clients {
		stream, err := bc.NewClientSubscriptionStream(c.name, c.database)
		if err != nil {
			t.Errorf("error creating clientStream for cl %s", c.name)
		}
		outStreams[c.name] = stream
	}

	bc.EvictClient("client2")
	for _, msgs := range pubMessages {
		for _, m := range msgs {
			bc.Broadcast(m)
		}
	}

	select {
	case <-time.After(1 * time.Second):
		t.Errorf("closed channel should have been selected not properly evicted")
	case l, ok := <-outStreams["client2"]:
		if ok && len(l) != 0 {
			t.Errorf("client not properly evicted")
		}
	}

}

func TestBroadcastToSubscriber_Close(t *testing.T) {
	ctx, done := context.WithCancel(context.Background())
	defer done()
	bc := broadcaster.NewBroadcaster(ctx)
	defer bc.Close()
	clients := []client{
		{
			name:     "client1",
			database: "database1",
		},
		{
			name:     "client2",
			database: "database2",
		},
	}
	outStreams := make(map[string]<-chan []*quicksilverpb.TransactionLogs)
	for _, c := range clients {
		stream, err := bc.NewClientSubscriptionStream(c.name, c.database)
		if err != nil {
			t.Errorf("error creating clientStream for cl %s", c.name)
		}
		outStreams[c.name] = stream
	}
	bc.Close()

	for _, str := range outStreams {
		select {
		case <-time.After(1 * time.Second):
			t.Errorf("closed channel should have been selected not properly closed")
		case l, ok := <-str:
			if ok && len(l) != 0 {
				t.Errorf("broadcaster not properly closed")
			}
		}
	}

	// adding new clientStream should error out in closed broadcaster.
	_, err := bc.NewClientSubscriptionStream("random", "c.database")
	if err == nil {
		t.Errorf("closed broadcaster should not allow any new client addition")
	}
}
