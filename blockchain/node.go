package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var node host.Host

const (
	PeersInfoFile = "peers.json" // this file will be used by nodes on bootup for peer discovery
	topicBlock    = "block"
	topicTx       = "transaction"
)

type NodePubSub struct {
	blockTopic *pubsub.Topic
	txTopic    *pubsub.Topic
	blockSub   *pubsub.Subscription
	txSub      *pubsub.Subscription
}

type PeersInfo map[string]string // peers maps a peer ID (string) to its full P2P address (string)

func StartNode(ctx context.Context, port int, id string, randseed int64, connectAddr string, isMiner bool) error {
	if port == 0 {
		logger.Error("Please provide a valid port!")
		return fmt.Errorf("Please provide a valid port!")
	} else if !CheckPortAvailability("localhost", port) {
		logger.Errorf("Port: %d not available!", port)
		return fmt.Errorf("Port: %d is available!", port)
	}

	var err error
	onlinePeers, err := ReadPeersInfo()
	if err != nil && err != io.EOF {
		logger.Errorf("Error reading peersInfo file: %v\n", err)
		return fmt.Errorf("Error reading peersInfo file: %v\n", err)
	}
	if _, exists := onlinePeers[id]; exists {
		logger.Errorf("Node with given id is already running.\n")
		return fmt.Errorf("Node with given id is already running.\n")
	}

	var priv crypto.PrivKey
	if id != "" {
		priv, err = LoadNodePrivKey(id)
		if err != nil {
			logger.Error("No node found for the given id: ", err)
			return err
		}
	} else {
		priv, err = GeneratePrivKeyForNode(randseed)
		if err != nil {
			logger.Error("Error generating private key for node: ", err)
			return err
		}
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)),
		libp2p.Identity(priv),
	}

	node, err = libp2p.New(opts...)
	if err != nil {
		logger.Error(err)
		return err
	}

	node.Network().Notify(conn_notifiee)
	fullAddr := node.Addrs()[0].String() + "/p2p/" + node.ID().String()
	logger.Successf("Node started at address: %s\n", fullAddr)

	if err = SaveNodePrivKey(node.ID(), priv); err != nil {
		return err
	}

	nodePS, err := applyPubSub(ctx, node)
	if err != nil {
		logger.Error(err)
	} else {
		logger.Success("PubSub applied to node")
	}

	// Initialise db and create required buckets in the db
	db, err := InitDB()
	if err != nil {
		return err
	}
	defer db.Close()

	bc, err = NewBlockchain(db)
	if err != nil {
		logger.Errorf("Error initialising blockchain: %v\n", err)
	} else {
		logger.Infof("Loaded %d blocks from DB\n", len(bc.Chain))
	}

	utxoSet, err = NewUTXOSet(db)
	if err != nil {
		logger.Errorf("Error initialising utxoSet: %v\n", err)
	} else {
		logger.Infof("Loaded UTXOSet from DB\n")
	}

	connectToPeer(ctx, node, connectAddr)
	if err := writePeerInfoToJSONFile(fullAddr, node.ID().String(), PeersInfoFile); err != nil {
		logger.Error(err)
		return err
	}

	blockReceiver := make(chan *Block)
	go blockReader(ctx, nodePS.blockSub, blockReceiver)
	go txReader(ctx, nodePS.txSub)

	// sync blocks from connected peers on node startup
	handleSyncRequests(node)

	// wallet req handlers
	utxoRequestHandler(node)
	txReqHandler(node, nodePS.txTopic)

	if isMiner {
		if id != "" {
			priv, err := LoadWalletPrivKey(id)
			if err != nil {
				return fmt.Errorf("Error loading keys for the miner: %v\n", err)
			}
			NodeWallet = ConstructWallet(priv)
			logger.Success("Loaded existing wallet for miner")
		} else {
			NodeWallet = NewWallet()
			logger.Success("Generated and saved new wallet for miner")
		}
		go MineBlocks(ctx, blockReceiver, nodePS.blockTopic)
	} else {
		// drain the blockReceiver channel in case there is no miner to signal for new incoming block
		go func() {
			for range blockReceiver {
			}
		}()
	}

	// Signal handling for proper shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGABRT, syscall.SIGTERM)

	select {
	case quitSig := <-quit:
		logger.Info("Received signal: ", quitSig)
		if err := removePeerInfoFromJSONFile(node.ID().String(), PeersInfoFile); err != nil {
			logger.Error("Error removing the peer address from json file:", err)
		}

		logger.Info("Cleaning Up...")
		if err := db.Close(); err != nil {
			logger.Errorf("Failed to close db: %v", err)
		} else {
			logger.Info("DB closed gracefully...")
		}

		if err := node.Close(); err != nil {
			logger.Errorf("Failed to close node: %v", err)
		} else {
			logger.Info("Node exited gracefully...")
		}
		return nil
	}
}

func applyPubSub(ctx context.Context, h host.Host) (*NodePubSub, error) {
	psOpts := []pubsub.Option{
		pubsub.WithMessageSigning(true),
	}

	ps, err := pubsub.NewGossipSub(ctx, h, psOpts...)
	if err != nil {
		logger.Errorf("Error setting up instance for pubsub: %v", err)
		return nil, err
	}

	ps.RegisterTopicValidator(topicBlock, bTopicValidator)
	ps.RegisterTopicValidator(topicTx, txTopicValidator)

	blockTopic, err := ps.Join(topicBlock)
	if err != nil {
		return nil, fmt.Errorf("Error joining to topic 'block': %v\n", err)
	}

	blockSub, err := blockTopic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("Error subscribing to topic 'block': %v\n", err)
	}

	txTopic, err := ps.Join(topicTx)
	if err != nil {
		return nil, fmt.Errorf("Error joingin to topic 'transaction': %v\n", err)
	}

	txSub, err := txTopic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf("Error subscribing to topic 'transaction': %v\n", err)
	}

	nodePS := &NodePubSub{
		blockTopic: blockTopic,
		txTopic:    txTopic,
		blockSub:   blockSub,
		txSub:      txSub,
	}

	return nodePS, nil
}

func bTopicValidator(ctx context.Context, pid peer.ID, msg *pubsub.Message) bool {
	if len(msg.Data) == 0 {
		logger.Error("Invalid message: empty data")
		return false
	}

	b := &Block{}

	if err := json.Unmarshal(msg.Data, b); err != nil {
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
	if err := json.Unmarshal(msg.Data, tx); err != nil {
		logger.Error("Invalid json format")
		return false
	}

	if !tx.isValid() {
		return false
	}

	return true
}

func blockReader(ctx context.Context, bSub *pubsub.Subscription, blockReceiver chan<- *Block) {
	for {
		bMsg, err := bSub.Next(ctx)
		if err != nil {
			logger.Error("Error receiving next message: ")
		}

		if bMsg.GetFrom() == node.ID() {
			continue
		}
		block := &Block{}

		json.Unmarshal(bMsg.Data, &block)
		logger.Infof("Received block with id: %s from: %s\n", block.Hash, bMsg.GetFrom())

		if !block.isValid() {
			continue
		}

		for _, tx := range block.TxData {
			if !tx.IsCoinbase {
				mempool.RemoveTransaction(tx.TxID)
			}
		}

		if err = bc.AddBlock(block); err != nil {
			logger.Errorf("Error adding block: %v\n", err)
		} else {
			logger.Successf("Block:[%d]:[%s] added", block.Height, block.Hash)
		}

		if err = utxoSet.Update(); err != nil {
			logger.Errorf("Error adding updating utxoSet: %v\n", err)
		} else {
			logger.Success("UTXOSet updated")
		}

		blockReceiver <- block
	}
}

func txReader(ctx context.Context, txSub *pubsub.Subscription) {
	for {
		txMsg, _ := txSub.Next(ctx)
		if txMsg.GetFrom() == node.ID() {
			continue
		}
		tx := &Transaction{}

		json.Unmarshal(txMsg.Data, &tx)
		logger.Infof("Received transaction with id: %s from %s\n", tx.TxID, txMsg.GetFrom())
		mempool.AddTransaction(tx)
	}
}

// connectToPeer establishes a connection between the host and a peer node with the provided connectAddr(if provided)
// else connects to a random peer from `PeersInfoFile`
func connectToPeer(ctx context.Context, h host.Host, connectAddr string) {
	var peerInfo *peer.AddrInfo
	if connectAddr != "" {
		fullAddr, _ := multiaddr.NewMultiaddr(connectAddr)
		peerInfo, _ = peer.AddrInfoFromP2pAddr(fullAddr)
	} else {
		peerInfo = GetRandomPeerInfo()
	}
	if peerInfo != nil {
		if err := h.Connect(ctx, *peerInfo); err != nil {
			logger.Errorf("Failed to connect to peer %s: %v", connectAddr, err)
		} else {
			logger.Info("Syncing blocks from peer...")
			syncBlocksFromPeer(node, peerInfo.ID)
		}
	}
}

// GetRandomPeerInfo is a helper function for getting peerInfo from a random peer from NodeList
func GetRandomPeerInfo() *peer.AddrInfo {
	peers, err := ReadPeersInfo()
	if err != nil && err != io.EOF {
		logger.Error(err)
		return nil
	}

	if len(peers) == 0 {
		logger.Warn("No peers are online")
		return nil
	}

	fullAddrs := make([]string, 0, len(peers))
	for _, addr := range peers {
		fullAddrs = append(fullAddrs, addr)
	}
	rng := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	randomPeerAddr := fullAddrs[rng.Intn(len(fullAddrs))]
	fullAddr, _ := multiaddr.NewMultiaddr(randomPeerAddr)
	peerInfo, _ := peer.AddrInfoFromP2pAddr(fullAddr)
	return peerInfo
}

// ReadPeersInfo reads nodes from `PeersInfoFile` (creating the file if it doesn't exist), returns PeersInfo or an error.
func ReadPeersInfo() (PeersInfo, error) {
	var peers PeersInfo
	file, err := os.OpenFile(PeersInfoFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&peers); err != nil {
		return nil, err
	}

	return peers, nil
}
