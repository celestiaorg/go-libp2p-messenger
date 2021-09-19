// TODO: Docs
// TODO: Moooar Tests

// TODO: Required features
//  * Broadcast(test)
//  * Peer Connected/Disconnected events

// TODO: Others
//  * Reasonable channel sizes and options for them

// TODO: API
//	 * Alternative API where user passes handlers instead of relying on channels
//   * Built-in Request/Response API
//   * Rework Close to wait till all messages are processed

// TODO: Metrics
//  * Collect Read/Write bandwidth usage
package messenger

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

var log = logging.Logger("msnger")

type Messenger struct {
	pids []protocol.ID
	host host.Host

	msgTp reflect.Type

	inbound       chan *msgWrap
	streamsIn     map[peer.ID]inet.Stream
	newStreamsIn  chan inet.Stream
	deadStreamsIn chan peer.ID

	outbound       chan *msgWrap
	streamsOut     map[peer.ID]inet.Stream
	newStreamsOut  chan inet.Stream
	deadStreamsOut chan peer.ID
	peersOut       map[peer.ID]chan *msgWrap

	ctx    context.Context
	cancel context.CancelFunc
}

func NewMessenger(host host.Host, opts ...Option) (*Messenger, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Messenger{
		host:           host,
		msgTp:          reflect.TypeOf(serde.PlainMessage{}),
		inbound:        make(chan *msgWrap, 32),
		streamsIn:      make(map[peer.ID]inet.Stream),
		newStreamsIn:   make(chan inet.Stream, 32),
		deadStreamsIn:  make(chan peer.ID, 32),
		outbound:       make(chan *msgWrap, 32),
		streamsOut:     make(map[peer.ID]inet.Stream),
		newStreamsOut:  make(chan inet.Stream, 32),
		deadStreamsOut: make(chan peer.ID, 32),
		peersOut:       make(map[peer.ID]chan *msgWrap),
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
