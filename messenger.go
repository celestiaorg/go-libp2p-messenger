package msngr

import (
	"context"
	"reflect"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

var log = logging.Logger("msngr")

// Messenger provides a simple API to send messages to multiple peers.
type Messenger struct {
	host host.Host
	pids []protocol.ID
	msgTp reflect.Type

	// fields below are used and protected in processIn
	inbound       chan *msgWrap
	newStreamsIn  chan inet.Stream
	deadStreamsIn chan inet.Stream
	streamsIn     map[peer.ID]map[inet.Stream]context.CancelFunc

	// fields below are used and protected by processOut
	outbound       chan *msgWrap
	newStreamsOut  chan inet.Stream
	deadStreamsOut chan inet.Stream
	streamsOut     map[peer.ID]map[inet.Stream]context.CancelFunc
	peersOut       map[peer.ID]chan *msgWrap

	events chan PeerEvent

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
		msgTp:          reflect.TypeOf(serde.PlainMessage{}),
		inbound:        make(chan *msgWrap, 32),
		newStreamsIn:   make(chan inet.Stream, 4),
		deadStreamsIn:  make(chan inet.Stream, 2),
		streamsIn:      make(map[peer.ID]map[inet.Stream]context.CancelFunc),
		outbound:       make(chan *msgWrap, 32),
		newStreamsOut:  make(chan inet.Stream, 4),
		deadStreamsOut: make(chan inet.Stream, 2),
		peersOut:       make(map[peer.ID]chan *msgWrap),
		streamsOut:     make(map[peer.ID]map[inet.Stream]context.CancelFunc),
		events:         make(chan PeerEvent, 8),
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

// Send sends the given message 'out' to the 'peer' to.
// The returned channel is always closed once the message is sent successfully or with an error.
// All messages are sent in a per peer queue, so the ordering of sent messages is guaranteed.
// In case the Messenger is given with a RoutedHost, It tries to connect to the peer, if not connected.
func (m *Messenger) Send(ctx context.Context, out serde.Message, to peer.ID) <-chan error {
	msg := &msgWrap{
		Message: out,
		to:      to,
		done:    make(chan error, 1),
	}
	m.send(ctx, msg)
	return msg.done
}

// Receive awaits for incoming messages from peers.
// It receives messages sent through both Send and Broadcast.
// Ti errors only if the given context 'ctx' is closed or when Messenger itself is close.
func (m *Messenger) Receive(ctx context.Context) (serde.Message, peer.ID, error) {
	select {
	case msg := <-m.inbound:
		return msg.Message, msg.from, nil
	case <-ctx.Done():
		return nil, "", ctx.Err()
	case <-m.ctx.Done():
		return nil, "", m.ctx.Err()
	}
}

// Broadcast sends the given message 'out' to each connected peer speaking the same protocol.
// WARNING: It should be used deliberately. Avoid use cases requiring message propagation to a whole protocol network,
//  not to flood the network with message duplicates. For such cases use libp2p.PubSub instead.
func (m *Messenger) Broadcast(ctx context.Context, out serde.Message) <-chan error {
	msg := &msgWrap{
		Message: out,
		bcast:   true,
		done:    make(chan error, 1),
	}
	m.send(ctx, msg)
	return msg.done
}

// PeerEvent points to a peer and to a latest connection state with it.
type PeerEvent struct {
	ID    peer.ID
	State inet.Connectedness // Can be Connected or NotConnected only
}

// Events notifies about connection state changes of peers.
// If messenger is started over Host with existing connections,
// for every existing peer there will be an event.
func (m *Messenger) Events() <-chan PeerEvent {
	return m.events
}

// Close stop the Messenger and unregisters further protocol handling on the Host.
func (m *Messenger) Close() error {
	m.cancel()
	m.deinit()
	return nil
}

func (m *Messenger) send(ctx context.Context, msg *msgWrap) {
	select {
	case m.outbound <- msg:
	case <-ctx.Done():
		msg.Done(m.ctx.Err())
	case <-m.ctx.Done():
		msg.Done(m.ctx.Err())
	}
}

type msgWrap struct {
	serde.Message

	bcast    bool
	from, to peer.ID
	done     chan error
}

func (msg *msgWrap) Done(err error) {
	if err != nil {
		msg.done <- err
	}
	if !msg.bcast {
		close(msg.done)
	}
}
