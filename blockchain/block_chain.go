package blockchain

type Chain struct {
	blocks []*Block
}

func (c *Chain) prevBlock() *Block {
	return c.blocks[len(c.blocks)-1]
}

func (c *Chain) AddBlock(data string) {
	prev := c.prevBlock()

	new := CreateBlock(data, prev.Hash)

	c.blocks = append(c.blocks, new)
}

func (c *Chain) Blocks() []*Block {
	return c.blocks
}

func InitBlockChain() *Chain {
	return &Chain{[]*Block{Genesis()}}
}
