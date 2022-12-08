package msngr

import (
	"context"
	"errors"
	"reflect"
	"sync"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/host"
	inet "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var log = logging.Logger("msngr")

// errClosed reports if messenger is closed.
var errClosed = errors.New("msngr: closed")

// Messenger provides a simple API to send messages to multiple peers.
type Messenger struct {
	host  host.Host
	pids  []protocol.ID
	msgTp reflect.Type

	// fields below are used and protected in processIn
	inbound       chan Message
	newStreamsIn  chan inet.Stream
	deadStreamsIn chan inet.Stream
	streamsIn     map[peer.ID]map[inet.Stream]context.CancelFunc

	// fields below are used and protected by processOut
	outbound       chan Message
	newStreamsOut  chan inet.Stream
	deadStreamsOut chan inet.Stream
	streamsOut     map[peer.ID]map[inet.Stream]context.CancelFunc
	peersOut       map[peer.ID]chan Message
	peersReqs      chan chan peer.IDSlice
	broadcastMu    sync.Mutex
	broadcast      chan Message
	broadcastPeers chan peer.IDSlice

	ctx    context.Context
	cancel context.CancelFunc
}

// New instantiates a Messenger.
// WithProtocols option is mandatory for at least one protocol.
// WithMessageType overrides default serde.PlainMessage.
func New(host host.Host, opts ...Option) (*Messenger, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Messenger{
		host:           host,
		msgTp:          reflect.TypeOf(PlainMessage{}),
		inbound:        make(chan Message, 32),
		newStreamsIn:   make(chan inet.Stream, 4),
		deadStreamsIn:  make(chan inet.Stream, 2),
		streamsIn:      make(map[peer.ID]map[inet.Stream]context.CancelFunc),
		outbound:       make(chan Message, 32),
		newStreamsOut:  make(chan inet.Stream, 4),
		deadStreamsOut: make(chan inet.Stream, 2),
		streamsOut:     make(map[peer.ID]map[inet.Stream]context.CancelFunc),
		peersOut:       make(map[peer.ID]chan Message),
		peersReqs:      make(chan chan peer.IDSlice),
		broadcast:      make(chan Message, 1),
		broadcastPeers: make(chan peer.IDSlice),
		ctx:            ctx,
		cancel:         cancel,
	}
	err := m.options(opts...)
	if err != nil {
		return nil, err
	}

	go m.processOut()
	go m.processIn()
	m.init()
	return m, nil
}

// Host is a getter for the Host.
func (m *Messenger) Host() host.Host {
	return m.host
}

// Send optimistically sends the given message 'out' to the peer 'to'.
// It errors in case the given ctx was closed or in case when Messenger is closed.
// All messages are sent in a per peer queue, so the ordering of sent messages is guaranteed.
// In case the Messenger is given with a RoutedHost, It tries to connect to the peer, if not connected.
func (m *Messenger) Send(ctx context.Context, out Message) error {
	select {
	case m.outbound <- out:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-m.ctx.Done():
		return errClosed
	}
}

// Receive awaits for incoming messages from peers.
// It receives messages sent through both Send and Broadcast.
// It errors only if the given context 'ctx' is closed or when Messenger is closed.
func (m *Messenger) Receive(ctx context.Context) (Message, error) {
	select {
	case msg := <-m.inbound:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.ctx.Done():
		return nil, errClosed
	}
}

// Broadcast optimistically sends the given message 'out' to each connected peer
// speaking *the same* protocol. It returns a slice of peers to whom the message
// was sent and errors in case the given ctx was closed or in case when
// Messenger is closed. WARNING: It should be used deliberately. Avoid use cases
// requiring message propagation to a whole protocol network, not to flood the
// network with message duplicates. For such cases use libp2p.PubSub instead.
func (m *Messenger) Broadcast(ctx context.Context, out Message) (peer.IDSlice, error) {
	// ensures synchronization between broadcasting and broadcastPeers
	m.broadcastMu.Lock()
	defer m.broadcastMu.Unlock()

	select {
	case m.broadcast <- out:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.ctx.Done():
		return nil, errClosed
	}

	// wait for the peers we sent msgs to
	// NOTE: the global response channel is used to avoid allocations of some msg wrapper struct
	//  with response chan
	select {
	case peers := <-m.broadcastPeers:
		return peers, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.ctx.Done():
		return nil, errClosed
	}
}

// Peers returns a list of connected/immediate peers speaking the protocol registered on the Messenger.
func (m *Messenger) Peers() peer.IDSlice {
	req := make(chan peer.IDSlice, 1)
	select {
	case m.peersReqs <- req:
		select {
		case peers := <-req:
			return peers
		case <-m.ctx.Done():
			return nil
		}
	case <-m.ctx.Done():
		return nil
	}
}

// Close stops the Messenger and unregisters further protocol handling on the Host.
func (m *Messenger) Close() error {
	m.cancel()
	m.deinit()
	return nil
}
