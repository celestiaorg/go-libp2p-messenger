package msngr

import (
	"github.com/celestiaorg/go-libp2p-messenger/serde"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Message is a contract that any user defined message has to satisfy to be sent via Messenger.
type Message interface {
	serde.Message
	// From points to peer the Message was received from.
	From() peer.ID
	// To points to Messenger the peer to send the Message to.
	To() peer.ID
}

// NewMessageFn is a type-parameterized constructor func for Message.
// It is required by Messenger, s.t. it can instantiate user-defined messages
// coming from the network.
type NewMessageFn[M Message] func(from, to peer.ID) M
