package blockchain

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"golang.org/x/net/context"
)

func HandleNodeCommands(ctx context.Context, txPublisher *pubsub.Topic) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf(Blue + "\n-> " + Reset)
		command, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}

		command = strings.TrimSpace(command)
		switch {
		case strings.HasPrefix(command, "sendCoin"):
			args := strings.Split(command, " ")
			if len(args) < 3 {
				fmt.Println("Usage: sendtx <receipent> <amount>")
				continue
			}
			// yet to be implemented...
			// if err != nil {
			// 	fmt.Println("Error sending transaction:", err)
			// } else {
			// 	fmt.Println("Transaction sent successfully!")
			// }

		case command == "shownodeaddr":
			fmt.Println("Node Address:", node.Addrs()[0].String()+"/p2p/"+node.ID().String())

		case command == "showwalletaddr":
			fmt.Println("Wallet Address:", NodeWallet.Address)

		case command == "showbal":
			fmt.Println(utxoSet.GetTotalBalByAddress(NodeWallet.Address))

		case command == "blocks":
			for _, b := range bc.Chain {
				spew.Dump(b.TxData)
			}

		default:
			fmt.Println("Unknown command. Available commands: sendCoin, shownodeaddr, showwalletaddr, showbal")
		}
	}
}
