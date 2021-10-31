package msngr

import (
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

func (m *Messenger) init() {
	for _, pid := range m.pids {
		m.host.SetStreamHandler(pid, m.streamIn)
	}
	m.host.Network().Notify(m)
	for _, c := range m.host.Network().Conns() {
		m.Connected(nil, c)
	}
}

func (m *Messenger) deinit() {
	for _, pid := range m.pids {
		m.host.RemoveStreamHandler(pid)
	}
}

func (m *Messenger) connect(p peer.ID) {
	// TODO: Retry with backoff several times and clean up outbound channel

	// assuming Host is wrapped with RoutedHost
	err := m.host.Connect(m.ctx, peer.AddrInfo{ID: p})
	if err != nil {
		log.Errorw("connecting", "to", p.ShortString(), "err", err)
	}
}

func (m *Messenger) reconnect(p peer.ID) {
	for _, c := range m.host.Network().ConnsToPeer(p) {
		if c.Stat().Transient {
			continue
		}

		m.Connected(nil, c)
		return
	}

	m.connect(p)
}

func (m *Messenger) Connected(_ network.Network, c network.Conn) {
	if c.Stat().Transient {
		return
	}

	go m.streamOut(c.RemotePeer())
}

func (m *Messenger) Disconnected(_ network.Network, _ network.Conn) {}

func (m *Messenger) Listen(_ network.Network, _ ma.Multiaddr) {}

func (m *Messenger) ListenClose(_ network.Network, _ ma.Multiaddr) {}

func (m *Messenger) OpenedStream(_ network.Network, _ network.Stream) {}

func (m *Messenger) ClosedStream(_ network.Network, _ network.Stream) {}
