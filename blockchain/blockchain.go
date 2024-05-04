package blockchain

import (
	"encoding/hex"
	"fmt"
	"os"
	"runtime"

	"github.com/Dimashey/blockchain/internal/util"
	"github.com/dgraph-io/badger"
)

const dbPath = "./tmp/blocks"

// Manifest file which badger creates when intialize database
const dbFile = "./tmp/blocks/MANIFEST"
const genesisData = "First Transaction from Genesis"

type Chain struct {
	LastHash []byte
	Database *badger.DB
}

func (c *Chain) lastHash() ([]byte, error) {

	var lastHash []byte

	err := c.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))

		util.HandleError(err)

		lastHash, err = item.ValueCopy(nil)

		return err
	})

	if err != nil {
		return nil, err
	}

	return lastHash, nil
}

func (c *Chain) AddBlock(txs []*Transaction) {
	lastHash, err := c.lastHash()

	util.HandleError(err)

	newBlock := CreateBlock(txs, lastHash)

	err = c.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())

		util.HandleError(err)

		err = txn.Set([]byte("lh"), newBlock.Hash)

		c.LastHash = newBlock.Hash

		return err
	})

	util.HandleError(err)
}

func DBexists() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}

func InitBlockChain(address string) *Chain {
	var lastHash []byte

	if DBexists() {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}

	opts := badger.DefaultOptions(dbPath)

	db, err := badger.Open(opts)

	util.HandleError(err)

	err = db.Update(func(txn *badger.Txn) error {
		// Check if blockchain is exists
		if _, err := txn.Get([]byte("lh")); err == badger.ErrKeyNotFound {
			cbtx := CoinbaseTx(address, genesisData)
			genesis := Genesis(cbtx)

			err = txn.Set(genesis.Hash, genesis.Serialize())

			util.HandleError(err)

			err = txn.Set([]byte("lh"), genesis.Hash)

			lastHash = genesis.Hash

			return err
		}

		// Get last element hash in blockchain
		item, err := txn.Get([]byte("lh"))
		util.HandleError(err)

		lastHash, err = item.ValueCopy(nil)

		return err
	})

	return &Chain{lastHash, db}
}

func ContinueBlockChain(address string) *Chain {
	if DBexists() == false {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}

	var lastHash []byte

	opts := badger.DefaultOptions(dbFile)

	db, err := badger.Open(opts)

	util.HandleError(err)

	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))

		util.HandleError(err)

		lastHash, err = item.ValueCopy(nil)

		return err
	})

	util.HandleError(err)

	chain := Chain{lastHash, db}

	return &chain
}

// FindUnspentTransactions finds all unsped transaction which belongs for address
func (c *Chain) FindUnspentTransactions(address string) []Transaction {
	var unspentTxs []Transaction

	spentTXOs := make(map[string][]int)

	iter := c.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, out := range tx.Outputs {
				if spentTXOs[txID] != nil {
					for _, stxoIdx := range spentTXOs[txID] {
						if stxoIdx == outIdx {
							continue Outputs
						}
					}
				}

				if out.CanBeUnlocked(address) {
					unspentTxs = append(unspentTxs, *tx)
				}
			}

			if tx.IsCoinbase() == false {
				for _, txIn := range tx.Inputs {
					if txIn.CanUnlock(address) {
						txInId := hex.EncodeToString(txIn.ID)
						spentTXOs[txInId] = append(spentTXOs[txInId], txIn.Out)
					}
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return unspentTxs
}

// FindUTXO finds all unsped transaction outputs which belongs for address
// FYI: UTXO it TxOutput which is not used by other input what means
// they form user balance
func (c *Chain) FindUTXO(address string) []TxOutput {
	var utxos []TxOutput
	utx := c.FindUnspentTransactions(address)

	for _, tx := range utx {
		for _, out := range tx.Outputs {
			if out.CanBeUnlocked(address) {
				utxos = append(utxos, out)
			}
		}
	}

	return utxos
}

func (c *Chain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	unspentTxs := c.FindUnspentTransactions(address)
	accumulated := 0

Work:
	for _, tx := range unspentTxs {
		txId := hex.EncodeToString(tx.ID)

		for outIdx, out := range tx.Outputs {
			if out.CanBeUnlocked(address) && accumulated < amount {
				accumulated += out.Value
				unspentOuts[txId] = append(unspentOuts[txId], outIdx)

				if accumulated >= amount {
					break Work
				}
			}
		}
	}

	return accumulated, unspentOuts
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

// Iterator returns iterator which go through blockchain in reverse order from last to genesis block
func (c *Chain) Iterator() *BlockChainIterator {
	iter := &BlockChainIterator{c.LastHash, c.Database}

	return iter
}

func (iter *BlockChainIterator) Next() *Block {
	var block *Block

	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		util.HandleError(err)

		encodedBlock, err := item.ValueCopy(nil)
		block = Deserialize(encodedBlock)

		return err
	})

	util.HandleError(err)

	iter.CurrentHash = block.PrevHash

	return block
}
