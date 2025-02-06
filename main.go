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
	availableModes := []string{"fullnode", "twallet"}
	mode := flag.String("mode", "", "Specify the mode.")
	listModes := flag.Bool("list-modes", false, "List available modes")

	port := flag.Int("p", 0, "Port on which the node will listen")
	target := flag.String("t", "", "Multiaddr of the peer to connect to (if left empty will connect to a random mutliaddress from nodes.json file)")
	seed := flag.Int64("s", 0, "Seed for random peer ID")
	flag.Parse()

	if *listModes {
		fmt.Println("Available modes: ")
		for _, m := range availableModes {
			fmt.Println(m)
		}
		os.Exit(0)
	}

	switch *mode {
	case availableModes[0]:
		fmt.Println(Blue, "Starting full node...", Reset)

		err := blockchain.StartNode(context.Background(), *port, *seed, *target)
		if err != nil {
			fmt.Println(Red, "Cannot connect to node!!!", Reset)
			os.Exit(1)
		}
	case availableModes[1]:
		fmt.Println(Blue, "Starting wallet interface...", Reset)
		if *port == 0 {
			fmt.Println(Red, "ERROR: Please Specify a valid for the wallet to listen at with '-p' flag", Reset)
			os.Exit(1)
		} else if !blockchain.CheckPortAvailability("localhost", *port) {
			fmt.Println(Red, "ERROR: Port is busy, provide another availaible port", Reset)
			os.Exit(1)
		}

	default:
		fmt.Println(Red, "Please specify a valid mode with '-mode' flag. Use -list-modes to see available modes", Reset)
		os.Exit(1)
	}
}
