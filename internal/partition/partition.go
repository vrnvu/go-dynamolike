package partition

import (
	"bytes"

	"github.com/dgryski/go-farm"
	"github.com/lithammer/go-jump-consistent-hash"
)

type Partitioner interface {
	Hash(key string) int
}

type Partition struct {
	nodes        int
	virtualNodes int
	hasher       *jump.Hasher
}

type FarmHash struct {
	buf bytes.Buffer
}

func (f *FarmHash) Write(p []byte) (n int, err error) {
	return f.buf.Write(p)
}

func (f *FarmHash) Reset() {
	f.buf.Reset()
}

func (f *FarmHash) Sum64() uint64 {
	// https://github.com/dgryski/go-farm
	return farm.Hash64(f.buf.Bytes())
}

func New(nodes int) *Partition {
	virtualNodes := nodes * 1000
	hasher := jump.New(virtualNodes, &FarmHash{})
	return &Partition{
		nodes:        nodes,
		virtualNodes: virtualNodes,
		hasher:       hasher,
	}
}

func (p *Partition) Hash(key string) int {
	return p.hasher.Hash(key) % p.nodes
}
