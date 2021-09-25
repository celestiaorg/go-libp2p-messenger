package msngr

import (
	"context"
	"math/rand"
	"testing"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

func TestMessengerSimpleSendCustomType(t *testing.T) {
	logging.SetLogLevel("messenger", "debug")

	const tproto protocol.ID = "/test"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mnet, err := mocknet.FullMeshConnected(ctx, 2)
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
}

//
// func TestMessengerBroadcast(t *testing.T) {
// 	const tproto protocol.ID = "/test"
// 	const netSize = 5
//
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
//
// 	mnet, err := mocknet.FullMeshConnected(ctx, netSize)
// 	require.NoError(t, err)
//
// 	ms := make([]*Messenger, netSize)
// 	for i, h := range mnet.Hosts() {
// 		ms[i], err = New(h, WithProtocols(tproto))
// 		require.NoError(t, err)
// 	}
//
// 	for _, m := range ms {
// 		m.Broadcast(ctx, randPlainMessage(100))
//
// 	}
// }

func randPlainMessage(size int) *serde.PlainMessage {
	msg := &serde.PlainMessage{Data: make([]byte, size)}
	rand.Read(msg.Data)
	return msg
}
