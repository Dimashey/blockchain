package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"

	"github.com/Dimashey/blockchain/internal/util"
)

type Transaction struct {
	ID      []byte
	Inputs  []TxInput
	Outputs []TxOutput
}

func (tx *Transaction) SetID() {
	var encoded bytes.Buffer
	var hash [32]byte

	encode := gob.NewEncoder(&encoded)
	err := encode.Encode(tx)

	util.HandleError(err)

	hash = sha256.Sum256(encoded.Bytes())
	tx.ID = hash[:]
}

type TxOutput struct {
	// Value in tokens assing to specific address
	// Example 1 BTC
	Value int
	// User address to which token was sent. FYI this simplified verion
	PubKey string
}

func (txOut *TxOutput) CanBeUnlocked(data string) bool {
	return txOut.PubKey == data
}

type TxInput struct {
	// Reference to Tx from whic we get TxOut
	ID []byte
	// Index of TxOut. For exampe transaction can have 4 TxOut we need reference only one of them
	Out int
	// Simlar to TxOutput.PubKey
	Sig string
}

// CanUnlock provides proof of ownership or authorization to spend funds.
func (txIn *TxInput) CanUnlock(data string) bool {
	return txIn.Sig == data
}

// CoinbaseTx create first transaction in blockchain network
func CoinbaseTx(to, data string) *Transaction {
	if data == "" {
		data = fmt.Sprintf("Coin to %s", to)
	}

	txIn := TxInput{[]byte{}, -1, data}
	txOut := TxOutput{100, to}

	tx := Transaction{nil, []TxInput{txIn}, []TxOutput{txOut}}
	tx.SetID()

	return &tx
}

func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Inputs) == 1 && len(tx.Inputs[0].ID) == 0 && tx.Inputs[0].Out == -1
}
