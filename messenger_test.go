package msngr

import (
	"context"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	bhost "github.com/libp2p/go-libp2p/p2p/host/blank"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	swarmt "github.com/libp2p/go-libp2p/p2p/net/swarm/testing"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multistream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

const tproto protocol.ID = "/test"

// NOTE: Add the line below to any tests to see all logs:
// 	logging.SetLogLevel("msngr", "debug")
// To see logs for whole libp2p stack use:
//  logging.SetDebugLogging()

// Checks that messenger is send a message, if peers are connected.
func TestSend_PeersConnected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// create network with connected peers
	mnet, err := mocknet.FullMeshConnected(2)
	require.NoError(t, err)

	min, err := New(mnet.Hosts()[0], WithProtocols(tproto))
	require.NoError(t, err)

	mout, err := New(mnet.Hosts()[1], WithProtocols(tproto))
	require.NoError(t, err)

	msgin := randPlainMessage(256, mnet.Peers()[1])
	err = min.Send(ctx, msgin)
	require.NoError(t, err)

	msgout, err := mout.Receive(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, msgin.Data, msgout.(*PlainMessage).Data)
	assert.Equal(t, mnet.Hosts()[0].ID(), msgout.From())

	err = min.Close()
	require.NoError(t, err)
	err = mout.Close()
	require.NoError(t, err)
}

// Checks that messenger is able to connect and send a message, if peers are not connected.
func TestSend_PeersDisconnected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// create network with linked, but disconnected peers
	mnet, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)

	min, err := New(mnet.Hosts()[0], WithProtocols(tproto))
	require.NoError(t, err)

	mout, err := New(mnet.Hosts()[1], WithProtocols(tproto))
	require.NoError(t, err)

	msgin := randPlainMessage(256, mnet.Peers()[1])
	err = min.Send(ctx, msgin)
	require.NoError(t, err)

	msgout, err := mout.Receive(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, msgin.Data, msgout.(*PlainMessage).Data)
	assert.Equal(t, mnet.Hosts()[0].ID(), msgout.From())

	err = min.Close()
	require.NoError(t, err)
	err = mout.Close()
	require.NoError(t, err)
}

func TestReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	hosts := realTransportHosts(t, 2)

	min, err := New(hosts[0], WithProtocols(tproto))
	require.NoError(t, err)

	mout, err := New(hosts[1], WithProtocols(tproto))
	require.NoError(t, err)

	err = hosts[0].Connect(ctx, *host.InfoFromHost(hosts[1]))
	require.NoError(t, err)

	// wait some time
	time.Sleep(time.Millisecond * 100)

	go func() {
		for i := range make([]int, 10) {
			if i == 8 {
				err = hosts[0].Network().ClosePeer(hosts[1].ID())
				require.NoError(t, err)
			}
			min.Send(ctx, randPlainMessage(256, hosts[1].ID()))
		}
	}()

	for i := range make([]int, 9) {
		_, err := mout.Receive(ctx)
		if !assert.NoError(t, err) {
			t.Log(i)
			return
		}
	}
}

func TestStreamDuplicates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	hosts := realTransportHosts(t, 2)

	min, err := New(hosts[0], WithProtocols(tproto))
	require.NoError(t, err)

	_, err = New(hosts[1], WithProtocols(tproto))
	require.NoError(t, err)

	err = hosts[0].Connect(ctx, *host.InfoFromHost(hosts[1]))
	require.NoError(t, err)

	// wait some time
	time.Sleep(time.Millisecond * 100)

	tcp, err := tcp.NewTCPTransport(swarmt.GenUpgrader(t, hosts[1].Network().(*swarm.Swarm)), nil)
	require.NoError(t, err)

	var addr multiaddr.Multiaddr
	for _, a := range hosts[0].Addrs() {
		ps := a.Protocols()
		if ps[len(ps)-1].Code == multiaddr.P_TCP {
			addr = a
			break
		}
	}

	err = hosts[0].Network().ClosePeer(hosts[1].ID())
	require.NoError(t, err)

	conn, err := tcp.Dial(ctx, addr, hosts[0].ID())
	require.NoError(t, err)

	// check sending on duplicate
	out, err := conn.OpenStream(ctx)
	require.NoError(t, err)

	err = multistream.SelectProtoOrFail(string(tproto), out)
	require.NoError(t, err)

	msgout := randPlainMessage(256, "")
	_, err = serde.Write(out, msgout)
	require.NoError(t, err)

	msgin, err := min.Receive(ctx)
	require.NoError(t, err)
	assert.Equal(t, hosts[1].ID(), msgin.From())
	assert.Equal(t, msgout.Data, msgin.(*PlainMessage).Data)

	// // check receiving on duplicate
	sin, err := conn.AcceptStream()
	require.NoError(t, err)

	ms := multistream.NewMultistreamMuxer()
	ms.AddHandler(string(tproto), func(protocol string, rwc io.ReadWriteCloser) error {
		_, err = serde.Read(rwc, msgin)
		require.NoError(t, err)
		assert.Equal(t, msgout.Data, msgin.(*PlainMessage).Data)
		return nil
	})

	_, h, err := ms.Negotiate(sin)
	require.NoError(t, err)

	msgout = randPlainMessage(256, hosts[1].ID())
	err = min.Send(ctx, msgout)
	require.NoError(t, err)

	err = h("", sin)
	require.NoError(t, err)
}

func TestSend_Events(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	mnet, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)

	firstHst := mnet.Hosts()[0]
	firstSub, err := firstHst.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	require.NoError(t, err)

	first, err := New(mnet.Hosts()[0], WithProtocols(tproto))
	require.NoError(t, err)

	secondHst := mnet.Hosts()[1]
	secondSub, err := secondHst.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	require.NoError(t, err)

	second, err := New(mnet.Hosts()[1], WithProtocols(tproto))
	require.NoError(t, err)

	_, err = mnet.ConnectPeers(mnet.Peers()[0], mnet.Peers()[1])
	require.NoError(t, err)

	evt := (<-firstSub.Out()).(event.EvtPeerConnectednessChanged)
	assert.Equal(t, evt.Peer, secondHst.ID())
	assert.Equal(t, evt.Connectedness, network.Connected)

	evt = (<-secondSub.Out()).(event.EvtPeerConnectednessChanged)
	assert.Equal(t, evt.Peer, firstHst.ID())
	assert.Equal(t, evt.Connectedness, network.Connected)

	err = first.Send(ctx, randPlainMessage(256, secondHst.ID()))
	assert.NoError(t, err)
	err = second.Send(ctx, randPlainMessage(256, firstHst.ID()))
	assert.NoError(t, err)

	msgout, err := first.Receive(ctx)
	require.NoError(t, err)
	assert.Equal(t, secondHst.ID(), msgout.From())
	msgout, err = second.Receive(ctx)
	require.NoError(t, err)
	assert.Equal(t, firstHst.ID(), msgout.From())

	err = first.Close()
	require.NoError(t, err)
	err = firstSub.Close()
	require.NoError(t, err)
	err = second.Close()
	require.NoError(t, err)
	err = secondSub.Close()
	require.NoError(t, err)
}

func TestGroupBroadcast(t *testing.T) {
	const netSize = 4

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mnet, err := mocknet.FullMeshConnected(netSize)
	require.NoError(t, err)

	// create messengers according to netSize
	ms := make([]*Messenger, netSize)
	for i, h := range mnet.Hosts() {
		ms[i], err = New(h, WithProtocols(tproto))
		require.NoError(t, err)
	}

	// have to wait till everyone ready
	time.Sleep(time.Millisecond * 100)

	// do actual broadcasting
	for _, m := range ms {
		peers, err := m.Broadcast(ctx, randPlainMessage(100, ""))
		require.NoError(t, err)
		assert.Len(t, peers, netSize-1)
	}

	// actually check everyone received a message from everyone
	for _, m := range ms {
		for range ms[1:] {
			_, err := m.Receive(ctx) // we don't really care about the content, rather about the fact of receival
			assert.NoError(t, err)
		}
	}

	// have to wait till everyone received
	time.Sleep(time.Millisecond * 100)

	// be nice and close
	for _, m := range ms {
		err = m.Close()
		require.NoError(t, err)
	}
}

func TestPeers(t *testing.T) {
	const netSize = 4

	mnet, err := mocknet.FullMeshLinked(netSize)
	require.NoError(t, err)

	// create messengers according to netSize
	ms := make([]*Messenger, netSize)
	for i, h := range mnet.Hosts() {
		ms[i], err = New(h, WithProtocols(tproto))
		require.NoError(t, err)
	}

	err = mnet.ConnectAllButSelf()
	require.NoError(t, err)

	// have to wait till everyone ready
	time.Sleep(time.Millisecond * 100)

	for _, m := range ms {
		peers := m.Peers()
		assert.Len(t, peers, netSize-1)
	}
}

func randPlainMessage(size int, to peer.ID) *PlainMessage {
	msg := &PlainMessage{}
	msg.Data = make([]byte, size)
	msg.to = to
	rand.Read(msg.Data)
	return msg
}

func realTransportHosts(t *testing.T, n int) []host.Host {
	out := make([]host.Host, n)
	for i := range out {
		netw := swarmt.GenSwarm(t)
		h := bhost.NewBlankHost(netw)
		out[i] = h
	}

	return out
}
