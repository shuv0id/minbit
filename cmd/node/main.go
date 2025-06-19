package main

import (
	"context"
	"fmt"
	"os"

	"github.com/shu8h0-null/minbit/core"
	"github.com/shu8h0-null/minbit/core/logger"
	"github.com/shu8h0-null/minbit/core/netstack"
	"github.com/shu8h0-null/minbit/core/rpc"
	"github.com/urfave/cli/v3"
)

var log = logger.NewLogger()

func main() {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:     "port",
				Aliases:  []string{"p"},
				Usage:    "Port for node to listen at",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "id",
				Aliases: []string{"id"},
				Value:   "",
				Usage:   "ID of the node to start",
			},
			&cli.StringFlag{
				Name:    "target",
				Aliases: []string{"t"},
				Value:   "",
				Usage:   "Multiaddr of the peer to connect to (if left empty will connect to a random mutliaddress from peers.json file)",
			},
			&cli.Int64Flag{
				Name:    "seed",
				Aliases: []string{"s"},
				Value:   0,
				Usage:   "Seed for random peer ID",
			},
			&cli.BoolFlag{
				Name:  "mine",
				Value: false,
				Usage: "Enable mining",
			},
			// &cli.StringFlag{
			// 	Name:  "create-wallet",
			// 	Value: "",
			// 	Usage: "Create wallet with <wallet_name>. wallet.dat is saved at ./wallets/<wallet_name>/wallet.dat",
			// },
			// &cli.StringFlag{
			// 	Name:  "load-wallet",
			// 	Value: "",
			// 	Usage: "Path to load wallet from",
			// },
			&cli.BoolFlag{
				Name:  "serve",
				Value: false,
				Usage: "Expose rpc APIs",
			},
			&cli.StringFlag{
				Name:  "rpc-addr",
				Value: "",
				Usage: "Address of for rpc listener",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			port := cmd.Int("port")
			id := cmd.String("id")
			target := cmd.String("target")
			serve := cmd.Bool("serve")
			rpcAddr := cmd.String("rpc-addr")
			minerMode := cmd.Bool("mine")
			seed := cmd.Int64("seed")
			if !netstack.CheckPortAvailability("127.0.0.1", port) {
				return fmt.Errorf("Provied port: %s not available: %v", port)
			}

			if serve && rpcAddr == "" {
				return fmt.Errorf("Please provide an address for rpc.\n")
			}

			ctxB := context.Background()
			node, err := core.InitNode(ctxB, port, id, seed, minerMode)
			if err != nil {
				return fmt.Errorf("Error initialising node: %v", err)
			}

			err = node.Connect(ctxB, target)
			if err != nil && err != core.ErrNoOnlinePeers {
				return fmt.Errorf("Error connecting to the target:%v\n", err)
			}

			if serve {
				handler := rpc.NewRPCHandler(node)
				go func() {
					err := rpc.StartRPC(rpcAddr, handler)
					if err != nil {
						node.Close()
						log.Error(err)
					}
				}()
			}

			if err = node.Run(ctxB); err != nil {
				return fmt.Errorf("Error running the node:%v\n", err)
			}
			return nil
		},
	}
	err := cmd.Run(context.Background(), os.Args)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
