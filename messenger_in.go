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
	for {
		select {
		case s := <-m.newStreamsIn:
			p := s.Conn().RemotePeer()
			log.Debugw("new stream", "from", p.ShortString())

			_, ok := m.streamsIn[p]
			if ok {
				// duplicate? overwrite with the recent one
				log.Warnw("duplicate stream", "from", p.ShortString())
			}

			go m.msgsIn(s)
			m.streamsIn[p] = s
		case p := <-m.deadStreamsIn:
			// TODO: In case of stream duplicate this will delete a newest valid stream
			delete(m.streamsIn, p)
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
			case m.deadStreamsIn <- from:
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
