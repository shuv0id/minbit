package blockchain

type Transaction struct {
	TxID      string
	Sender    string
	Recipent  string
	Amount    int
	Signature string
	Inputs    []UTXO
	Outputs   []UTXO
}
