package msngr

import (
	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Message interface {
	serde.Message

	To() peer.ID
	From() peer.ID

	New(from, to peer.ID) Message
}

type PlainMessage struct {
	serde.PlainMessage

	from, to peer.ID
}

func (p *PlainMessage) To() peer.ID {
	return p.to
}

func (p *PlainMessage) From() peer.ID {
	return p.from
}

func (p *PlainMessage) New(from, to peer.ID) Message {
	return &PlainMessage{
		from: from,
		to:   to,
	}
}
