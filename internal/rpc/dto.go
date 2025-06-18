package rpc

type Result[T any] struct {
	Result T `json:"result"`
}

// eth_blockNumber
type LatestBlockDTO = Result[string]

// eth_getBlockReceipts
type BlockReceiptsDTO = Result[[]Receipt]

type Receipt struct {
	From              string  `json:"from"`
	To                string  `json:"to"`
	Status            string  `json:"status"` // 0x1 went through, 0x0 it failed
	GasUsed           string  `json:"gasUsed"`
	EffectiveGasPrice string  `json:"effectiveGasPrice"` // FffectiveGasPrice * GasUsed + l1Fee = fee
	TransactionHash   string  `json:"transactionHash"`   // Matches with Transaction.Hash
	L1Fee             *string `json:"l1Fee,omitempty"`   // Can be empty for system level transactions
}

// eth_getBlockByNumber
type BlockDTO = Result[BlockData]

type BlockData struct {
	Number       string        `json:"number"`
	Transactions []Transaction `json:"transactions"`
}

type Transaction struct {
	From  string `json:"from"`
	To    string `json:"to,omitempty"`
	Value string `json:"value"` // How much was moved
	Input string `json:"input"` // If it is plainly "0x", it is a transfer; otherwise, it's a contract call
	Hash  string `json:"hash"`
}

// eth_getBalance
type BalanceDTO = Result[string]
