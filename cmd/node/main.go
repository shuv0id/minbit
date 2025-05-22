package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/shu8h0-null/minbit/core"
	blkchn "github.com/shu8h0-null/minbit/core/blockchain"
	"github.com/shu8h0-null/minbit/core/logger"
	"github.com/shu8h0-null/minbit/core/netstack"
)

const (
	Reset       = "\033[0m"
	Red         = "\033[31m"
	Blue        = "\033[34m"
	blockBucket = "Blocks"
	utxoBucket  = "Utxos"
	maxTxCount  = 5
)

var log = logger.NewLogger()

func main() {
	port := flag.Int("p", 0, "Port on which the node will listen")
	target := flag.String("t", "", "Multiaddr of the peer to connect to (if left empty will connect to a random mutliaddress from peers.json file)")
	id := flag.String("id", "", "ID of the node to start")
	seed := flag.Int64("s", 0, "Seed for random peer ID")
	minerMode := flag.Bool("mine", false, "Whether the node can mine blocks")
	flag.Parse()

	if *port <= 0 {
		log.Errorf("Please provide a valid port!")
		os.Exit(1)
	} else if !netstack.CheckPortAvailability("localhost", *port) {
		log.Errorf("Port: %d not available!", *port)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listenForQuitSignal(ctx, cancel)

	blockCh := make(chan *blkchn.Block)
	node, err := initNode(ctx, *port, *id, *seed, blockCh)
	if err != nil {
		log.Error("Error initialising node %v\n", err)
		os.Exit(1)
	}

	log.Info("Starting full node...")
	store, err := initStore(node.ID())
	if err != nil {
		log.Errorf("Could not initialise db for blockchain: %v\n", err)
		os.Exit(1)
	}

	cs, err := initChainState(store)
	if err != nil {
		log.Errorf("Error creating new chainstate: %v\n", err)
		os.Exit(1)
	}

	err = node.SetChainState(cs)
	if err != nil {
		log.Errorf("Error setting chainstate for node: %v\n", err)
		os.Exit(1)
	}

	addr := *target
	if addr == "" {
		var err error
		addr, err = node.GetRandomPeerAddr()
		if err != nil {
			log.Errorf("Error getting a random online peer address: %v\n", err)
		}
	}

	if addr != "" {
		if err := netstack.ConnectToPeer(ctx, node.Host(), addr); err != nil {
			log.Errorf("Error connecting to peer [%s]: %v\n", addr, err)
		} else {
			peerID, err := netstack.ExtractPeerID(addr)
			if err != nil {
				log.Errorf("Error extracting peer id from the peer address:[%s]: %v\n", addr, err)
				os.Exit(1)
			}
			err = node.SyncBlocksFromPeer(ctx, peerID, cs.Blockchain().GetBlockchainHeight())
			if err != nil {
				log.Errorf("Error syncing blocks: %v\n", err)
				os.Exit(1)
			}
		}
	}

	if *minerMode {
		minerWallet, err := blkchn.NewWallet()
		if err != nil {
			log.Errorf("Error creating wallet for miner %v\n", err)
			os.Exit(1)
		}

		miner, err := blkchn.NewMiner(minerWallet, blockCh)
		if err != nil {
			log.Errorf("Error initialising miner %v\n", err)
			os.Exit(1)
		}
		err = node.SetMiner(miner)
		if err != nil {
			log.Errorf("Error setting up miner for node %v\n", err)
			os.Exit(1)
		}
		go node.Mine(ctx)
	} else {
		for range blockCh {

		}
	}

	<-ctx.Done()
	log.Info("Cleaning Up...")
	if err = netstack.RemoveHostMultiaddrFromFile(node.ID()); err != nil {
		log.Errorf("Error removing host address from peers file: %v\n", err)
	}
	if err = node.Close(); err != nil {
		log.Errorf("Error closing node: %v\n", err)
	}
	if err = store.Close(); err != nil {
		log.Errorf("Error closing db: %v\n", err)
	}
	os.Exit(0)
}

func initNode(ctx context.Context, port int, id string, randseed int64, blockCh chan *blkchn.Block) (*core.Node, error) {
	var err error
	if id != "" {
		onlinePeers, err := netstack.ReadOnlinePeers()
		if err != nil && err != io.EOF {
			log.Errorf("Error reading peersInfo file: %v\n", err)
			return nil, fmt.Errorf("Error reading peersInfo file: %v\n", err)
		}
		if _, exists := onlinePeers[id]; exists {
			log.Errorf("Node with given id is already running.\n")
			return nil, fmt.Errorf("Node with given id is already running.\n")
		}
	}

	var priv crypto.PrivKey
	if id != "" {
		priv, err = netstack.LoadNodePrivKey(id)
		if err != nil {
			log.Error("No node found for the given id: ", err)
			return nil, err
		}
	} else {
		priv, err = netstack.GeneratePrivKeyForNode(randseed)
		if err != nil {
			log.Error("Error generating private key for node: ", err)
			return nil, err
		}
	}

	node, err := core.NewNode(context.Background(), port, priv)
	if err != nil {
		return nil, err
	}

	if err = netstack.SaveHostPrivKey(node.ID(), priv); err != nil {
		return nil, err
	}
	if err = netstack.SaveHostAddrToFile(node.Host()); err != nil {
		return nil, err
	}

	go node.TxReader(ctx)
	go node.BlockReader(ctx, blockCh)
	node.HandleSyncRequests()
	return node, err
}

func initStore(hostID peer.ID) (*blkchn.Store, error) {
	store, err := blkchn.NewDb(filepath.Join("store", hostID.String()))
	if err != nil {
		return nil, err
	}

	err = store.CreateBlocksBucket()
	if err != nil {
		return nil, err
	}

	err = store.CreateUTXOSetBucket()
	if err != nil {
		return nil, err
	}

	return store, nil
}

func initChainState(store *blkchn.Store) (*blkchn.ChainState, error) {
	bc, err := blkchn.NewBlockchain(store, blockBucket)
	if err != nil {
		return nil, fmt.Errorf("Could not create blockchain: %v\n", err)
	}

	us, err := blkchn.NewUTXOSet(store, utxoBucket)
	if err != nil {
		return nil, fmt.Errorf("Could not create utxoset: %v\n", err)
	}

	mem := blkchn.NewMempool()

	cs, err := blkchn.NewChainState(bc, us, mem)
	if err != nil {
		return nil, fmt.Errorf("Error creating new chainstate: %v\n", err)
	}

	return cs, nil
}

func listenForQuitSignal(ctx context.Context, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigCh:
			log.Infof("Received signal: %s, shutting down...", sig)
			cancel() // Trigger context cancellation
		}
	}()
}
