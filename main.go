package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/shu8h0-null/minimal-btc/blockchain"
)

const (
	Reset = "\033[0m"
	Red   = "\033[31m"
	Blue  = "\033[34m"
)

func main() {
	port := flag.Int("p", 0, "Port on which the node will listen")
	target := flag.String("t", "", "Multiaddr of the peer to connect to (if left empty will connect to a random mutliaddress from nodes.json file)")
	id := flag.String("id", "", "ID of the node to start")
	seed := flag.Int64("s", 0, "Seed for random peer ID")
	miner := flag.Bool("miner", false, "Whether the node can mine blocks")
	flag.Parse()

	fmt.Println(Blue, "Starting full node...", Reset)

	err := blockchain.StartNode(context.Background(), *port, *id, *seed, *target, *miner)
	if err != nil {
		os.Exit(1)
	}
}
