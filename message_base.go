package msngr

import "github.com/libp2p/go-libp2p/core/peer"

// MessageBase is a helper to struct that users may embed into their custom Message implementations.
// It's sole purpose is to minimize boilerplate code.
type MessageBase struct {
	FromPeer, ToPeer peer.ID
}

func (mb *MessageBase) From() peer.ID {
	return mb.FromPeer
}

func (mb *MessageBase) To() peer.ID {
	return mb.ToPeer
}
