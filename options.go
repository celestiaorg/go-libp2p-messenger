package msngr

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/protocol"
)

type options struct {
	pids []protocol.ID
}

// Option defines an functional configurability for Messenger.
type Option func(*options)

// WithProtocols sets custom protocols for messenger to speak with
// At least one protocol is required.
func WithProtocols(pids ...protocol.ID) Option {
	return func(o *options) {
		o.pids = append(o.pids, pids...)
	}
}

func parseOptions(opts ...Option) (*options, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	if o.pids == nil {
		return nil, fmt.Errorf("messenger: at least one protocol must be given through an option")
	}
	return o, nil
}
