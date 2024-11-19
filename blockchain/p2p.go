package blockchain

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	mrand "math/rand"
	"strings"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"

	// "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	// "github.com/libp2p/go-libp2p/core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

func startNode(port int, secio bool, randseed int64) error {
	var r io.Reader
	if randseed == 0 {
		r = rand.Reader
	} else {
		r = mrand.New(mrand.NewSource(randseed))
	}

	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/%d", port)),
		libp2p.Identity(priv),
	}

	basicHost, err := libp2p.New(opts...)
	if err != nil {
		return err
	}

	basicHost.SetStreamHandler("/p2p/1.0.0", handleStream)

	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", basicHost.ID().String()))

	addrs := basicHost.Addrs()
	var addr ma.Multiaddr

	for _, a := range addrs {
		if strings.HasPrefix(a.String(), "/ip4") {
			addr = a
			break
		}
	}

	fullAddr := addr.Encapsulate(hostAddr)

	log.Printf("Host started at address: %s", fullAddr)

	return nil
}

func handleStream(s network.Stream) {
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
	go readData(rw)
	go writeData(rw)
}

func readData(rw *bufio.ReadWriter) {
	for {
		// yet to be implemented
	}
}

func writeData(rw *bufio.ReadWriter) {
	// yet to be implemented
}
