package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/shu8h0-null/minbit/core/blockchain"
	"github.com/urfave/cli/v3"
)

type RPCClient struct {
	GetBlockByHash   func(hash string) *blockchain.Block
	GetBlockByHeight func(height uint64) *blockchain.Block
}

func main() {
	var client RPCClient
	closer, err := jsonrpc.NewClient(
		context.Background(),
		"http://localhost:8080/rpc/v0",
		"NodeRPC",
		&client,
		nil,
	)
	if err != nil {
		panic(err)
	}

	defer closer()
	cmd := &cli.Command{
		Commands: []*cli.Command{
			{
				Name:  "get-block",
				Usage: "get block information",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "hash",
						Value: "",
						Usage: "hash of the block to query",
					},
					&cli.IntFlag{
						Name:  "height",
						Value: -1,
						Usage: "height of the block to query",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.String("hash") != "" {
						block := client.GetBlockByHash(cmd.String("hash"))
						jsonBytes, err := json.MarshalIndent(block, "", " ")
						if err != nil {
							fmt.Println("Error marshalling received block to json", err)
						}
						fmt.Println(string(jsonBytes))
					}

					if cmd.Int("height") != -1 {
						block := client.GetBlockByHeight(uint64(cmd.Int("height")))
						jsonBytes, err := json.MarshalIndent(block, "", " ")
						if err != nil {
							fmt.Println("Error marshalling received block to json", err)
						}
						fmt.Println(string(jsonBytes))
					}
					return nil
				},
			},
			// {
			// 	Name: "createwallet",
			// 	Action: func(ctx context.Context, cmd *cli.Command) error {
			// 		if cmd.String("createwallet") != "" {
			//
			// 		}
			// 		return nil
			// 	},
			// },
		},
	}

	if cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
