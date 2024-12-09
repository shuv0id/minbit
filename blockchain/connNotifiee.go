package blockchain

import (
	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
)

type MyNotifiee struct{}

func (n *MyNotifiee) Listen(net network.Network, addr ma.Multiaddr) {}

func (n *MyNotifiee) ListenClose(net network.Network, addr ma.Multiaddr) {}

func (n *MyNotifiee) Connected(net network.Network, conn network.Conn) {
	logger.Info("Connected to peer: %s\n", conn.RemotePeer())
}

func (n *MyNotifiee) Disconnected(net network.Network, conn network.Conn) {
	logger.Info("Disconnected from peer: %s\n", conn.RemotePeer())
}
