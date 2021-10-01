package msngr

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

const tproto protocol.ID = "/test"

// NOTE: Add the line below to any tests to see all logs:
// 	logging.SetLogLevel("msngr", "debug")
// To see logs for whole libp2p stack use:
//  logging.SetDebugLogging()

func TestSendPeersConnected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// create network with connected peers
	mnet, err := mocknet.FullMeshConnected(ctx, 2)
	require.NoError(t, err)

	min, err := New(mnet.Hosts()[0], WithProtocols(tproto))
	require.NoError(t, err)

	mout, err := New(mnet.Hosts()[1], WithProtocols(tproto))
	require.NoError(t, err)

	msgin := randPlainMessage(256)
	done := min.Send(ctx, msgin, mnet.Peers()[1])
	require.NoError(t, <-done)

	msgout, from, err := mout.Receive(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, msgin, msgout)
	assert.Equal(t, mnet.Hosts()[0].ID(), from)

	err = min.Close()
	require.NoError(t, err)
	err = mout.Close()
	require.NoError(t, err)
}

// Checks that messenger is able to connect and send a message, if peers are not connected.
func TestSendPeersDisconnected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// create network with linked, but disconnected peers
	mnet, err := mocknet.FullMeshLinked(ctx, 2)
	require.NoError(t, err)

	min, err := New(mnet.Hosts()[0], WithProtocols(tproto), WithMessageType(&serde.PlainMessage{}))
	require.NoError(t, err)

	mout, err := New(mnet.Hosts()[1], WithProtocols(tproto), WithMessageType(&serde.PlainMessage{}))
	require.NoError(t, err)

	msgin := randPlainMessage(256)
	done := min.Send(ctx, msgin, mnet.Peers()[1])
	require.NoError(t, <-done)

	msgout, from, err := mout.Receive(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, msgin, msgout)
	assert.Equal(t, mnet.Hosts()[0].ID(), from)

	err = min.Close()
	require.NoError(t, err)
	err = mout.Close()
	require.NoError(t, err)
}

func TestEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	mnet, err := mocknet.FullMeshLinked(ctx, 3)
	require.NoError(t, err)

	// preconnect third node with first and second
	_, err = mnet.ConnectPeers(mnet.Peers()[2], mnet.Peers()[0])
	require.NoError(t, err)
	_, err = mnet.ConnectPeers(mnet.Peers()[2], mnet.Peers()[1])
	require.NoError(t, err)

	firstId := mnet.Hosts()[0].ID()
	first, err := New(mnet.Hosts()[0], WithProtocols(tproto))
	require.NoError(t, err)

	secondId := mnet.Hosts()[1].ID()
	second, err := New(mnet.Hosts()[1], WithProtocols(tproto))
	require.NoError(t, err)

	thirdId := mnet.Hosts()[2].ID()
	_, err = New(mnet.Hosts()[2], WithProtocols(tproto))
	require.NoError(t, err)

	// ensure first and second receive an event for a preconnected third node.
	evt := <-first.Events()
	assert.Equal(t, evt.ID, thirdId)
	assert.Equal(t, evt.State, network.Connected)
	evt = <-second.Events()
	assert.Equal(t, evt.ID, thirdId)
	assert.Equal(t, evt.State, network.Connected)

	// connect first and second now
	err = first.Host().Connect(ctx, *host.InfoFromHost(second.host))
	require.NoError(t, err)

	// check that both received an event
	evt = <-first.Events()
	assert.Equal(t, evt.ID, secondId)
	assert.Equal(t, evt.State, network.Connected)
	evt = <-second.Events()
	assert.Equal(t, evt.ID, firstId)
	assert.Equal(t, evt.State, network.Connected)

	// close the connection between first and second
	conns := first.Host().Network().ConnsToPeer(secondId)
	require.NotEmpty(t, conns)
	err = conns[0].Close()
	require.NoError(t, err)

	// close all the connections of third
	conns = mnet.Hosts()[2].Network().Conns()
	require.NotEmpty(t, conns)
	err = conns[0].Close()
	require.NoError(t, err)
	err = conns[1].Close()
	require.NoError(t, err)

	// check for the NotConnected event
	evt = <-first.Events()
	assert.Equal(t, evt.ID, secondId)
	assert.Equal(t, evt.State, network.NotConnected)
	evt = <-second.Events()
	assert.Equal(t, evt.ID, firstId)
	assert.Equal(t, evt.State, network.NotConnected)
	evt = <-first.Events()
	assert.Equal(t, evt.ID, thirdId)
	assert.Equal(t, evt.State, network.NotConnected)
	evt = <-second.Events()
	assert.Equal(t, evt.ID, thirdId)
	assert.Equal(t, evt.State, network.NotConnected)

	err = first.Close()
	require.NoError(t, err)
	err = second.Close()
	require.NoError(t, err)

	_, ok := <-first.Events()
	assert.False(t, ok)
	_, ok = <-second.Events()
	assert.False(t, ok)
}

func TestGroupBroadcast(t *testing.T) {
	const netSize = 4

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mnet, err := mocknet.FullMeshConnected(ctx, netSize)
	require.NoError(t, err)

	// create messengers according to netSize
	ms := make([]*Messenger, netSize)
	for i, h := range mnet.Hosts() {
		ms[i], err = New(h, WithProtocols(tproto))
		require.NoError(t, err)
	}

	// ensure everyone is connected
	for _, m := range ms {
		for range ms[1:] {
			<-m.Events()
		}
	}

	// do actual broadcasting
	for _, m := range ms {
		m.Broadcast(ctx, randPlainMessage(100))
	}

	// actually check everyone received a message from everyone
	for _, m := range ms {
		for range ms[1:] {
			_, _, err := m.Receive(ctx) // we don't really care about the content, rather about the fact of receival
			assert.NoError(t, err)
		}
	}

	// be nice and close
	for _, m := range ms {
		err = m.Close()
		require.NoError(t, err)
	}
}

func randPlainMessage(size int) *serde.PlainMessage {
	msg := &serde.PlainMessage{Data: make([]byte, size)}
	rand.Read(msg.Data)
	return msg
}
