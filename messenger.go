package messenger

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"reflect"
	"sync/atomic"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/celestiaorg/go-libp2p-messenger/serde/serdetest"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p-core/host"
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
)

var log = logging.Logger("messenger")

// TODO: Consensus handle multiple channels
//  * Multiple messengers per consensus
//  * Multiple streams per messenger

// TODO: MsgOut should be reworked to have idependent loops
// TODO: Consider moving to a more framework like design
// TODO: Broadcast
// TODO: Stream lifecycle
// TODO: Messenger lifecycle
// TODO: Peer Connected/Disconnected
// TODO: Handle ID
// TODO: Channel sizes
// TODO: Metrics

type Message struct {
	serde.Message

	ID uint64
	From, To peer.ID
	Broadcast bool
	Done chan error
}

type Messenger struct {
	seqno uint64

	pids []protocol.ID
	host host.Host

	streamsOut   map[peer.ID]inet.Stream
	streamsIn    map[peer.ID]inet.Stream
	newStreamsIn chan inet.Stream
	deadStreamsIn chan peer.ID

	inbound chan *Message
	outbound chan *Message

	msgTp reflect.Type

	ctx context.Context
	cancel context.CancelFunc
}

func NewMessenger(host host.Host, pids ...protocol.ID) *Messenger {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Messenger{
		pids:         pids,
		host:         host,
		streamsIn:    make(map[peer.ID]inet.Stream),
		streamsOut:   make(map[peer.ID]inet.Stream),
		newStreamsIn: make(chan inet.Stream, 32),
		inbound:      make(chan *Message),
		outbound:     make(chan *Message),
		msgTp:        reflect.TypeOf(serdetest.FakeMessage{}),
		ctx:          ctx,
		cancel:       cancel,
	}
	for _, pid := range pids {
		host.SetStreamHandler(pid, func(s inet.Stream) {
			select {
			case m.newStreamsIn <- s:
			case <-m.ctx.Done():
				return
			}
		})
	}
	go m.outboundLoop1()
	go m.handleStreams()
	return m
}

func (m *Messenger) Send(ctx context.Context, out serde.Message, to peer.ID) {
	done := make(chan error, 1)
	msg := &Message{
		Message: out,
		ID: atomic.AddUint64(&m.seqno, 1),
		From: m.host.ID(),
		To: to,
		Done: done,
	}
	select {
	case m.outbound <- msg:
	case <-ctx.Done():
		done <- ctx.Err()
	case <-m.ctx.Done():
		done <- ctx.Err()
	}
}

func (m *Messenger) Receive(ctx context.Context) (*Message, error) {
	select {
	case msg := <-m.inbound:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *Messenger) Inbound() <-chan *Message {
	return m.inbound
}

func (m *Messenger) Outbound() chan<- *Message {
	return m.outbound
}

func (m *Messenger) Close() error {
	m.cancel()
	return nil
}

func (m *Messenger) outboundLoop1() {
	for {
		select {
		case msg := <-m.outbound:
			log.Debugw("msgOut", "id", msg.ID, "to", msg.To.ShortString())
			msg.Done <- m.msgOut(msg)
			close(msg.Done)
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *Messenger) inboundLoop(s inet.Stream) {
	r := bufio.NewReader(s)
	for {
		msg, err := m.readMsg(r)
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Error(err)
			s.Reset()
			return
		}

		select {
		case m.inbound <- msg:
		case <-m.ctx.Done():
			s.Reset()
			return
		default:
			log.Warnf("Dropped msg from %s - slow reader.", msg.From.ShortString())
		}
	}
}

func (m *Messenger) handleStreams() {
	for {
		select {
		case s := <-m.newStreamsIn:
			peer := s.Conn().RemotePeer()
			log.Debugw("new stream from", "peer", peer.ShortString())
			_, ok := m.streamsIn[peer]
			if ok {
				// duplicate? maybe a retry
				log.Warnf("stream duplicate for peer %s", peer.ShortString())
				// the last one is the valid, so proceed
			}

			go m.inboundLoop(s)
			m.streamsIn[peer] = s
		case p := <-m.deadStreamsIn:
			delete(m.streamsIn, p)
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *Messenger) msgOut(msg *Message) error {
	s, err := m.streamOut(msg.To)
	if err != nil {
		return err
	}

	// TODO:
	//  * Metrics
	//  * Reties
	log.Debugw("writeMsg", "id", msg.ID, "to", msg.To.ShortString())
	err = m.writeMsg(s, msg)
	if err != nil {
		s.Reset()
		return err
	}
	return nil
}

func (m *Messenger) streamOut(p peer.ID) (inet.Stream, error) {
	s, ok := m.streamsOut[p]
	if ok {
		return s, nil
	}

	s, err := m.host.NewStream(m.ctx, p, m.pids...)
	if err != nil {
		return nil, err // TODO: Meaningful error
	}

	log.Debugw("new stream to", "peer", p.ShortString())
	m.streamsOut[p] = s
	return s, nil
}

// TODO: Collect Read/Write bandwidth usage?

func (m *Messenger) readMsg(r io.Reader) (*Message, error) {
	msg := &Message{Message: reflect.New(m.msgTp).Interface().(serde.Message)}
	_, err := serde.Read(r, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to read message from peer %s: %w", msg.From.ShortString(), err)
	}
	return msg, nil
}

func (m *Messenger) writeMsg(w io.Writer, msg *Message) error {
	_, err := serde.Write(w, msg)
	if err != nil {
		return fmt.Errorf("failed to write message to peer %s: %s", msg.To.ShortString(), err)
	}
	return nil
}
