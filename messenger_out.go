package msngr

import (
	"context"

	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

// streamOut stands for outbound streams creation.
func (m *Messenger) streamOut(p peer.ID) {
	s, err := m.host.NewStream(m.ctx, p, m.pids...)
	if err != nil {
		// it is a normal case when a peer does not speak the same protocol while we connected to him
		log.Debugw("error opening new stream to peer", "peer", p.ShortString(), "err", err)
		return
	}

	select {
	case m.newStreamsOut <- s:
	case <-m.ctx.Done():
		s.Reset()
	}
}

// processOut means processing everything related to outbound data.
func (m *Messenger) processOut() {
	defer func() {
		close(m.events)
		for p := range m.streamsOut {
			delete(m.streamsOut, p)
		}
		for p := range m.peersOut {
			delete(m.peersOut, p)
		}
	}()

	fire := func(evt PeerEvent) {
		select {
		case m.events <- evt:
		default:
			log.Warnf("event dropped(Slow Events reader)")
		}
	}

	for {
		select {
		case msg := <-m.outbound:
			if msg.bcast {
				for _, out := range m.peersOut {
					select {
					case out <- msg:
					case <-m.ctx.Done():
						return
					}
				}
				continue
			}

			out, ok := m.peersOut[msg.to]
			if !ok {
				out = make(chan *msgWrap, 32)
				m.peersOut[msg.to] = out
				// if no outbound chan for the peer, then assume there is no respective stream and thus connection
				go m.connect(msg.to)
			}

			select {
			// TODO: The case where we failed to create a stream to a non connected peer is possible here and
			//  once the channel is full this will block forever
			case out <- msg:
			case <-m.ctx.Done():
				return
			}

		case s := <-m.newStreamsOut:
			p := s.Conn().RemotePeer()
			log.Debugw("new stream", "to", p.ShortString())

			ss, ok := m.streamsOut[p]
			if !ok {
				ss = make(map[inet.Stream]context.CancelFunc)
				m.streamsOut[p] = ss
			}

			if len(ss) == 0 {
				fire(PeerEvent{ID: p, State: inet.Connected})
			} else {
				// duplicate? we use the recent stream only
				for _, cancel := range ss {
					cancel()
				}
				log.Warnw("duplicate stream", "to", p.ShortString())
			}

			out, ok := m.peersOut[p]
			if !ok {
				out = make(chan *msgWrap, 32)
				m.peersOut[p] = out
			}

			ctx, cancel := context.WithCancel(m.ctx)
			go m.msgsOut(ctx, s, out)
			ss[s] = cancel
		case s := <-m.deadStreamsOut:
			p := s.Conn().RemotePeer()

			ss := m.streamsOut[p]
			delete(ss, s)
			if len(ss) != 0 {
				// cleanup of an original stream in case of a duplicate
				continue
			}
			fire(PeerEvent{ID: p, State: inet.NotConnected})

			// TODO: This is the place where we could also cleanup outbound chan,
			//  but the reason for peer being dead might be a short term disconnect,
			//  so instead of dropping all the pending messages in the chan, we should give them some time to live
			//  and only after the time passes - drop.
			//
			//  NOTE: There is a chance for a first out message to be dropped due to reconnect,
			//  as msgsOut will read from the out chan and fail with msg reset. For this to be fixed more advanced
			//  queue should be used instead of native Go chan.

		case <-m.ctx.Done():
			return
		}
	}
}

// msgsOut handles outbound peer stream lifycycle and writes outgoing messages handed from processOut
func (m *Messenger) msgsOut(ctx context.Context, s inet.Stream, out <-chan *msgWrap) {
	closed := make(chan struct{})
	go func() {
		// a valid trick to check if stream is closed/reset
		// once Read unblocked then its closed/reset
		_, err := serde.Read(s, &serde.PlainMessage{})
		if err == nil {
			s.Reset()
			log.Warnw("totally unexpected message", "from", s.Conn().RemotePeer().ShortString())
		}

		close(closed)
		select {
		case m.deadStreamsOut <- s:
		case <-m.ctx.Done():
		}
	}()

	defer s.Close()
	for {
		select {
		case msg := <-out: // out is not going to be closed, thus no 'ok' check
			_, err := serde.Write(s, msg)
			msg.Done(err)
			if err != nil {
				log.Errorw("writing message", "to", msg.to.ShortString(), "err", err)
				s.Reset()
				return
			}
		case <-closed:
			return
		case <-ctx.Done():
			return
		}
	}
}
