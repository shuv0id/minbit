package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/davecgh/go-spew/spew"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

var node host.Host

var bootstrapNodes []host.Host

func StartNode(port int, secio bool, randseed int64, connectAddr string) error {
	priv, err := GeneratePrivKeyForNode(randseed)
	if err != nil {
		return err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)),
		libp2p.Identity(priv),
	}

	node, err = libp2p.New(opts...)
	if err != nil {
		return err
	}

	// subscribing
	nodePS, err := applyPubSub(node)
	if err != nil {
		logger.Errorf("Error setting up instance for pubsub: %v", err)
		return err
	}

	nodePS.RegisterTopicValidator("block", bTopicValidator)
	nodePS.RegisterTopicValidator("transaction", txTopicValidator)

	bTopic, err := nodePS.Join("block")
	if err != nil {
		logger.Errorf("Error subscribing to topic 'block': %v", err)
		return err
	}
	bSub, _ := bTopic.Subscribe()

	txTopic, err := nodePS.Join("transaction")
	if err != nil {
		logger.Errorf("Error subscribing to topic 'transaction': %s\n", err)
		return err
	}
	txSub, _ := txTopic.Subscribe()

	blockReceiver := make(chan *Block, 1)

	go bReader(bSub, blockReceiver)
	go txReader(txSub)

	node.Network().Notify(&MyNotifiee{})

	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", node.ID().String()))

	addrs := node.Addrs()
	var addr ma.Multiaddr

	for _, a := range addrs {
		if strings.HasPrefix(a.String(), "/ip4") {
			addr = a
			break
		}
	}

	fullAddr := addr.Encapsulate(hostAddr)
	logger.Successf("Node started at address: %s\n", fullAddr)

	go MineBlocks(blockReceiver, bTopic)

	if connectAddr != "" {
		peerAddr, err := ma.NewMultiaddr(connectAddr)
		if err != nil {
			logger.Errorf("Invalid peer multiaddr: %s\n", connectAddr)
		}

		peerInfo, err := peer.AddrInfoFromP2pAddr(peerAddr)
		if err != nil {
			logger.Errorf("Failed to get AddrInfo: %v\n", err)
		}

		err = node.Connect(context.Background(), *peerInfo)
		if err != nil {
			logger.Errorf("Error connecting to %s : %v\n", peerInfo.ID, err)
		}
	}

	done := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		logger.Info("Received signal: ", sig)
		close(done)
	}()

	select {
	case <-done:
		logger.Error("Node shutting down...")
		node.Close()
		return nil
	}
}

func applyPubSub(h host.Host) (*pubsub.PubSub, error) {
	psOpts := []pubsub.Option{
		pubsub.WithMessageSigning(true),
	}

	return pubsub.NewGossipSub(context.Background(), h, psOpts...)
}

func bTopicValidator(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
	if len(msg.Data) == 0 {
		logger.Error("Invalid message: empty data")
		return false
	}

	b := &Block{}
	err := json.Unmarshal(msg.Data, b)
	if err != nil {
		logger.Error("Invalid json format")
		return false
	}

	return true
}

func txTopicValidator(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
	if len(msg.Data) == 0 {
		logger.Error("Invalid message: empty data")
		return false
	}

	tx := &Transaction{}
	err := json.Unmarshal(msg.Data, tx)
	if err != nil {
		logger.Error("Invalid json format")
		return false
	}

	if !tx.isValid() {
		return false
	}

	return true
}

func bReader(bSub *pubsub.Subscription, bReceiver chan<- *Block) {
	for {
		bMsg, err := bSub.Next(context.Background())
		if err != nil {
			logger.Error("Error receiving next message: ")
		}

		if bMsg.GetFrom() == node.ID() {
			continue
		}
		b := &Block{}

		json.Unmarshal(bMsg.Data, &b)
		logger.Infof("Received block with id: %s from: %s\n", b.Hash, bMsg.GetFrom())
		fmt.Println(Yellow)
		spew.Dump(b)
		fmt.Println(Reset)

		if !b.isValid() {
			continue
		}

		// Signals miner with receiving block
		bReceiver <- b

		bc.AddBlock(b)

		fmt.Println(Yellow, "recevied block added")
		spew.Dump(bc.Chain)
		fmt.Println(Reset)

		mempool.RemoveTransaction(b.TxData.TxID)

	}
}

func txReader(txSub *pubsub.Subscription) {
	for {
		txMsg, _ := txSub.Next(context.Background())
		if txMsg.GetFrom() == node.ID() {
			continue
		}
		tx := &Transaction{}

		json.Unmarshal(txMsg.Data, &tx)
		logger.Infof("Received transaction with id: %s from %s\n", tx.TxID, txMsg.GetFrom())
		mempool.AddTransaction(tx)
	}
}
