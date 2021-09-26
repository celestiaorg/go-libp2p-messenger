package msngr

import (
	"bufio"
	"errors"
	"io"
	"reflect"

	inet "github.com/libp2p/go-libp2p-core/network"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

func (m *Messenger) streamIn(s inet.Stream) {
	select {
	case m.newStreamsIn <- s:
	case <-m.ctx.Done():
		return
	}
}

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
				ss = make(map[inet.Stream]struct{})
				m.streamsIn[p] = ss
			} else if len(ss) != 0 {
				// duplicate? overwrite with the recent one
				log.Warnw("duplicate stream", "from", p.ShortString())
			}

			go m.msgsIn(s)
			ss[s] = struct{}{}
		case s := <-m.deadStreamsIn:
			delete(m.streamsIn[s.Conn().RemotePeer()], s)
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *Messenger) msgsIn(s inet.Stream) {
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
			case <-m.ctx.Done():
				return
			}

			if errors.Is(err, io.EOF) {
				return
			}
			log.Errorw("reading message", "from", from.ShortString(), "err", err)
			s.Reset()
			return
		}

		select {
		case m.inbound <- msg:
		case <-m.ctx.Done():
			return
		default:
			log.Warnw("message dropped (slow Receive reader)", "from", from.ShortString())
		}
	}
}
