package netstack

import (
	"github.com/libp2p/go-libp2p/core/network"
)

var connNotifiee = &network.NotifyBundle{
	ConnectedF: func(n network.Network, c network.Conn) {
		log.Infof("Connected to peer: %s\n", c.RemotePeer())
	},
	DisconnectedF: func(n network.Network, c network.Conn) {
		log.Infof("Disconnected from peer: %s\n", c.RemotePeer())
	},
}
