package blockchain

// import "github.com/davecgh/go-spew/spew"

type Block struct {
	Index      int
	TxData     Transaction
	Timestamps string
	Nonce      int
	Hash       string
	PrevHash   string
}

type Blockchain struct {
	Chain      []*Block
	Difficulty int
}

var bc Blockchain

func NewBlockchain() *Blockchain {
	genesisBlock := &Block{}
	genesisBlock = &Block{
		Hash: genesisBlock.calculateHash(),
	}

	bc.Chain = append(bc.Chain, genesisBlock)

	return &bc
}
