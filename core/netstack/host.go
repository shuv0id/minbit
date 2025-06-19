package netstack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/shu8h0-null/minbit/core/logger"
)

const (
	onlinePeers = "peers.json" // this file will be used by nodes on bootup for peer discovery
)

var log = logger.NewLogger()

type OnlinePeers map[string]string // peers maps a peer ID (string) to its full P2P address (string)

func NewHost(ctx context.Context, port int, priv crypto.PrivKey) (host.Host, error) {
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)),
		libp2p.Identity(priv),
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	h.Network().Notify(connNotifiee)

	return h, nil
}

// ConnectToPeer establishes a connection between the host and a peer node with the provided peerAddr
func ConnectToPeer(ctx context.Context, h host.Host, peerAddr string) error {
	if peerAddr == "" {
		return errors.New("peer address cannot be empty")
	}
	fullAddr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		return err
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(fullAddr)
	if err != nil {
		return err
	}
	if err = h.Connect(ctx, *peerInfo); err != nil {
		return err
	}

	return nil
}

// ReadOnlinePeers reads peers from `onlinePeers`
func ReadOnlinePeers() (OnlinePeers, error) {
	var peers OnlinePeers
	file, err := os.OpenFile(onlinePeers, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&peers); err != nil {
		return nil, err
	}

	return peers, nil
}
