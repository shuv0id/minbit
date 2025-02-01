package main

import (
	"context"
	"flag"
	"os"

	"github.com/shu8h0-null/minimal-btc/blockchain"
)

func main() {
	logger := blockchain.NewColorLogger()
	port := flag.Int("p", 0, "Port on which the node will listen")
	target := flag.String("t", "", "Multiaddr of the peer to connect to (if left empty will connect to a random mutliaddress from nodes.json file)")
	seed := flag.Int64("s", 0, "Seed for random peer ID")
	flag.Parse()

	logger.Info("Starting node...")

	err := blockchain.StartNode(context.Background(), *port, *seed, *target)
	if err != nil {
		logger.Error("Cannot connect to node!!!")
		os.Exit(1)
	}
}
