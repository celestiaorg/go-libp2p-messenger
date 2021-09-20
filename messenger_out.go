package messenger

import (
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

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

func (m *Messenger) processOut() {
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

			_, ok := m.streamsOut[p]
			if ok {
				// duplicate? overwrite with the recent one
				log.Warnw("duplicate stream", "to", p.ShortString())
			} else {
				select {
				case m.events <- PeerEvent{ID: p, State: inet.Connected}:
				default:
					log.Warnf("event dropped(Slow Events reader)")
				}
			}

			out, ok := m.peersOut[p]
			if !ok {
				out = make(chan *msgWrap, 32)
				m.peersOut[p] = out
			}

			go m.msgsOut(s, out)
			m.streamsOut[p] = s
		case p := <-m.deadStreamsOut:
			// TODO: BUG - In case of stream duplicate it deletes a newest valid stream
			delete(m.streamsOut, p)
			// we still might be connected and if so respawn the outbound stream
			if m.host.Network().Connectedness(p) == inet.Connected {
				go m.streamOut(p)
			} else {
				select {
				case m.events <- PeerEvent{ID: p, State: inet.NotConnected}:
				default:
					log.Warnf("event dropped(Slow Events reader)")
				}
			}

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

func (m *Messenger) msgsOut(s inet.Stream, out <-chan *msgWrap) {
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
		case m.deadStreamsOut <- s.Conn().RemotePeer():
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
		case <-m.ctx.Done():
			return
		}
	}
}
