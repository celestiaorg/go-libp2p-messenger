package messenger

import (
	"github.com/libp2p/go-libp2p-core/network"
)

type Notify Messenger

func (n *Notify) Connected(net network.Network, c network.Conn) {

}


func (n *Notify) Listen(_ network.Network, _ interface{}) {}

func (n *Notify) ListenClose(_ network.Network, _ interface{}) {}

func (n *Notify) Disconnected(_ network.Network, _ network.Conn) {}

func (n *Notify) OpenedStream(_ network.Network, _ network.Stream) {}

func (n *Notify) ClosedStream(_ network.Network, _ network.Stream) {}
