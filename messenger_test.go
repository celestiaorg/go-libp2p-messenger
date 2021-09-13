package messenger

import (
	"context"
	"testing"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/protocol"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-libp2p-messenger/serde/serdetest"
)

func TestMessengerSimpleSend(t *testing.T) {
	logging.SetLogLevel("messenger", "debug")

	const tproto protocol.ID = "/test"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mnet, err := mocknet.FullMeshConnected(ctx, 2)
	require.NoError(t, err)

	min := NewMessenger(mnet.Hosts()[0], tproto)
	mout := NewMessenger(mnet.Hosts()[1], tproto)

	msgin := serdetest.RandFakeMessage(256)
	min.Send(ctx, msgin, mnet.Peers()[1])

	msgout := <-mout.Inbound()
	assert.EqualValues(t, msgin, msgout.Message)

	err = min.Close()
	require.NoError(t, err)
	err = mout.Close()
	require.NoError(t, err)
}

func TestMessenger(t *testing.T) {
	const tproto protocol.ID = "/test"
	const netSize = 3

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mnet, err := mocknet.FullMeshConnected(ctx, netSize)
	require.NoError(t, err)

	ms := make([]*Messenger, netSize)
	for i, h := range mnet.Hosts() {
		ms[i] = NewMessenger(h, tproto)
	}



	for _, m := range ms {
		err = m.Close()
		require.NoError(t, err)
	}
}
