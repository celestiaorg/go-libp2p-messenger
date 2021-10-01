package msngr

import (
	"fmt"
	"reflect"

	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
)

// Option defines an functional configurability for Messenger.
type Option func(messenger *Messenger)

// WithProtocols sets custom protocols for messenger to speak with
// At least one protocol is required.
func WithProtocols(pids ...protocol.ID) Option {
	return func(m *Messenger) {
		m.pids = pids
	}
}

// WithMessageType sets a custom message type for receiving messages.
// Otherwise Messenger would not know what type to return in Receive.
func WithMessageType(msg serde.Message) Option {
	return func(m *Messenger) {
		tp := reflect.TypeOf(msg)
		if tp.Kind() == reflect.Ptr {
			tp = tp.Elem()
		}
		m.msgTp = tp
	}
}

func (m *Messenger) options(opts ...Option) error {
	for _, opt := range opts {
		opt(m)
	}

	if m.pids == nil {
		return fmt.Errorf("messenger: at least one protocol must be given through an option")
	}
	return nil
}
