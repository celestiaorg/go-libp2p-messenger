package msngr

import (
	"bufio"
	"context"
	"errors"
	"io"
	"reflect"

	inet "github.com/libp2p/go-libp2p-core/network"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

// streamIn handles inbound streams from StreamHandler registered on the Host.
func (m *Messenger) streamIn(s inet.Stream) {
	select {
	case m.newStreamsIn <- s:
	case <-m.ctx.Done():
		return
	}
}

// processIn means processing everything related inbound data.
func (m *Messenger) processIn() {
	defer func() {
		for p := range m.streamsIn {
			delete(m.streamsIn, p)
		}
	}()
	for {
		select {
		case s := <-m.newStreamsIn:
			p := s.Conn().RemotePeer()
			log.Debugw("new stream", "from", p.ShortString())

			ss, ok := m.streamsIn[p]
			if !ok {
				ss = make(map[inet.Stream]context.CancelFunc)
				m.streamsIn[p] = ss
			} else if len(ss) != 0 {
				// duplicate? we use the recent stream only
				for _, cancel := range ss {
					cancel()
				}
				log.Warnw("duplicate stream", "from", p.ShortString())
			}

			ctx, cancel := context.WithCancel(m.ctx)
			go m.msgsIn(ctx, s)
			ss[s] = cancel
		case s := <-m.deadStreamsIn:
			delete(m.streamsIn[s.Conn().RemotePeer()], s)
		case <-m.ctx.Done():
			return
		}
	}
}

// msgsIn handles an inbound peer stream lifecycle and reads msgs from it handing them to inbound chan.
func (m *Messenger) msgsIn(ctx context.Context, s inet.Stream) {
	defer s.Close()
	r := bufio.NewReader(s)

	from := s.Conn().RemotePeer()
	var msg *msgWrap
	for {
		msg = &msgWrap{
			Message: reflect.New(m.msgTp).Interface().(serde.Message),
			from:    from,
		}
		_, err := serde.Read(r, msg.Message)
		if err != nil {
			select {
			case m.deadStreamsIn <- s:
			case <-ctx.Done():
				return
			}

			if errors.Is(err, io.EOF) || errors.Is(err, inet.ErrReset) {
				return
			}
			log.Errorw("reading message", "from", from.ShortString(), "err", err)
			s.Reset()
			return
		}

		select {
		case m.inbound <- msg:
		case <-ctx.Done():
			return
		default:
			log.Warnw("message dropped (slow Receive reader)", "from", from.ShortString())
		}
	}
}
