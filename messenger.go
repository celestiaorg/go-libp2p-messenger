// TODO: Docs

// TODO: Required features
//  * Make close to block until all messages are processed
//  * Stream per Message type

// TODO: API
//   * Events API is bad, the point below should fix it
//	 * Alternative API where user passes handlers instead of relying on channels
//   * Built-in Request/Response API

// TODO: Metrics
//  * Collect Read/Write bandwidth usage
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

type Messenger struct {
	pids []protocol.ID
	host host.Host

	msgTp reflect.Type

	inbound       chan *msgWrap
	streamsIn     map[peer.ID]map[inet.Stream]struct{}
	newStreamsIn  chan inet.Stream
	deadStreamsIn chan inet.Stream

	outbound       chan *msgWrap
	streamsOut     map[peer.ID]map[inet.Stream]struct{}
	newStreamsOut  chan inet.Stream
	deadStreamsOut chan inet.Stream
	peersOut       map[peer.ID]chan *msgWrap

	events chan PeerEvent

	ctx    context.Context
	cancel context.CancelFunc
}

func New(host host.Host, opts ...Option) (*Messenger, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Messenger{
		host:           host,
		msgTp:          reflect.TypeOf(serde.PlainMessage{}),
		inbound:        make(chan *msgWrap, 32),
		streamsIn:      make(map[peer.ID]map[inet.Stream]struct{}),
		newStreamsIn:   make(chan inet.Stream, 4),
		deadStreamsIn:  make(chan inet.Stream, 2),
		outbound:       make(chan *msgWrap, 32),
		streamsOut:     make(map[peer.ID]map[inet.Stream]struct{}),
		newStreamsOut:  make(chan inet.Stream, 4),
		deadStreamsOut: make(chan inet.Stream, 2),
		peersOut:       make(map[peer.ID]chan *msgWrap),
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

func (m *Messenger) Host() host.Host {
	return m.host
}

func (m *Messenger) Broadcast(ctx context.Context, out serde.Message) <-chan error {
	msg := &msgWrap{
		Message: out,
		bcast:   true,
		done:    make(chan error, 1),
	}
	m.send(ctx, msg)
	return msg.done
}

func (m *Messenger) Send(ctx context.Context, out serde.Message, to peer.ID) <-chan error {
	msg := &msgWrap{
		Message: out,
		to:      to,
		done:    make(chan error, 1),
	}
	m.send(ctx, msg)
	return msg.done
}

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

type PeerEvent struct {
	ID    peer.ID
	State inet.Connectedness // Connected or NotConnected only
}

// If messenger is started over Host with existing connections,
// for every existing peer there will be an event.
func (m *Messenger) Events() <-chan PeerEvent {
	return m.events
}

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
