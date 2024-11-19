package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/mr-tron/base58/base58"
	"golang.org/x/crypto/ripemd160"
)

func (bc *Blockchain) AddBlock(tx Transaction) {
	prevBlock := bc.Chain[len(bc.Chain)-1]
	block := &Block{}
	block.Index = prevBlock.Index + 1
	block.Timestamps = time.Now().String()
	block.TxData = tx
	block.PrevHash = prevBlock.Hash
	block.mineBlock()

	bc.Chain = append(bc.Chain, block)
}

func (b *Block) mineBlock() {
	for i := 0; ; i++ {
		b.Nonce = i
		hash := b.calculateHash()
		prefix := strings.Repeat("0", bc.Difficulty)

		if !strings.HasPrefix(hash, prefix) {
			fmt.Println("Gotta do more work!!!")
			time.Sleep(time.Second)
			continue
		} else {
			b.Hash = b.calculateHash()
			fmt.Println("Hell yeah!! Block is mined ")
			break
		}

	}
}

func (b Block) isValid() bool {
	prevBlock := bc.Chain[len(bc.Chain)-1]

	if prevBlock.Hash != b.Hash {
		return false
	}
	if !b.validateHash() {
		return false
	}
	if prevBlock.Index+1 != b.Index {
		return false
	}
	return true
}

func (b *Block) validateHash() bool {
	h := b.calculateHash()
	if h != b.Hash {
		return false
	}
	return true
}

func (b Block) calculateHash() string {
	data := strconv.Itoa(b.Index) + b.TxData.String() + b.Timestamps + strconv.Itoa(b.Nonce) + b.PrevHash

	hash := sha256.Sum256([]byte(data))

	return hex.EncodeToString(hash[:])
}

func (tx Transaction) String() string {
	return fmt.Sprintf("%s %s %d %s", tx.Sender, tx.Recipent, tx.Amount, tx.Signature)
}

func PublickKeyToAddress(pubAddress string) (string, error) {
	pubKeyBytes, err := base58.Decode(pubAddress)
	if err != nil {
		log.Fatal("Invalid public key")
		return "", err
	}
	sha256Hash := sha256.Sum256([]byte(pubKeyBytes))

	ripemdHash := ripemd160.New().Sum(sha256Hash[:])

	return base58.Encode(ripemdHash), nil
}
