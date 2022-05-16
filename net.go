package msngr

import (
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (m *Messenger[M]) init() {
	for _, pid := range m.pids {
		m.host.SetStreamHandler(pid, m.streamIn)
	}
	for _, c := range m.host.Network().Peers() {
		m.connected(c)
	}

	sub, err := m.host.EventBus().Subscribe(&event.EvtPeerConnectednessChanged{})
	if err != nil {
		log.Errorw("subscribing to peer connectedness event", "err", err)
		return
	}

	go func() {
		for {
			select {
			case evt := <-sub.Out():
				cevt := evt.(event.EvtPeerConnectednessChanged)
				switch cevt.Connectedness {
				case network.Connected:
					m.connected(cevt.Peer)
					// we don't care about NotConnected case, as it is handled by observing stream being reset
				}
			case <-m.ctx.Done():
				err := sub.Close()
				if err != nil {
					log.Errorw("closing peer connectedness event subscription", "err", err)
				}
				return
			}
		}
	}()
}

func (m *Messenger[M]) deinit() {
	for _, pid := range m.pids {
		m.host.RemoveStreamHandler(pid)
	}
}

// connect tries to connect to the given peer
func (m *Messenger[M]) connect(p peer.ID) {
	// TODO: Retry with backoff several times and clean up outbound channel if no success connecting

	// assuming Host is wrapped with RoutedHost
	err := m.host.Connect(m.ctx, peer.AddrInfo{ID: p})
	if err != nil {
		log.Errorw("connecting", "to", p.ShortString(), "err", err)
	}
}

// reconnect initiates new connection to the peer if not connected
func (m *Messenger[M]) reconnect(p peer.ID) {
	if m.connected(p) {
		return
	}
	m.connect(p)
}

// connected reports if there is at least one stable(non-transient) connection to the given peer
// and creates one stream to the peer if so.
func (m *Messenger[M]) connected(p peer.ID) bool {
	for _, c := range m.host.Network().ConnsToPeer(p) {
		if c.Stat().Transient {
			continue
		}
		log.Debugw("new peer", "id", p.ShortString())
		go m.streamOut(p)
		return true
	}

	return false
}
