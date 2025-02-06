package blockchain

import (
	"github.com/libp2p/go-libp2p/core/network"
)

var conn_notifiee = &network.NotifyBundle{
	ConnectedF: func(n network.Network, c network.Conn) {
		logger.Infof("Connected to peer: %s\n", c.RemotePeer())
	},
	DisconnectedF: func(n network.Network, c network.Conn) {
		logger.Infof("Disconnected from peer: %s\n", c.RemotePeer())
	},
}
