package automobile

import (
	"bufio"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
)

type BlockIndexEntry struct {
	Link  cid.Cid
	Start uint64
	End   uint64
}

type HeaderIndexEntry struct {
	Roots []cid.Cid
	Size  uint64
}

type indexerCfg struct {
	skipIntegrityChecks bool
}

type IndexerOption func(*indexerCfg)

// WithIndexSkipIntegrityChecks configures the indexer to skip integrity
// checks when reading blocks. This can be used to speed up indexing when the
// integrity of the blocks is not a concern.
func WithIndexerSkipIntegrityChecks() IndexerOption {
	return func(cfg *indexerCfg) {
		cfg.skipIntegrityChecks = true
	}
}

type Indexer struct {
	Header       HeaderIndexEntry
	reader       *bufio.Reader
	blockIndexer *BlockIndexer
}

func NewIndexer(r io.Reader, opts ...IndexerOption) (*Indexer, error) {
	cfg := &indexerCfg{}
	for _, opt := range opts {
		opt(cfg)
	}

	br := bufioReaderPool.Get().(*bufio.Reader)
	br.Reset(r)

	h, err := ReadHeader(br)
	if err != nil {
		bufioReaderPool.Put(br)
		return nil, err
	}

	if h.Version != 1 {
		return nil, fmt.Errorf("invalid CAR version: %d", h.Version)
	}

	offset, err := HeaderSize(h)
	if err != nil {
		return nil, err
	}

	headerEntry := HeaderIndexEntry{Roots: h.Roots, Size: offset}
	blr := BlockIndexer{Reader: br, Offset: offset, SkipIntegrityChecks: cfg.skipIntegrityChecks}
	return &Indexer{Header: headerEntry, reader: br, blockIndexer: &blr}, nil
}

func (r *Indexer) Read() (BlockIndexEntry, error) {
	ent, err := r.blockIndexer.Read()
	if err == io.EOF {
		// Common happy case: recycle the bufio.Reader.
		// In the other error paths leaking it is fine.
		bufioReaderPool.Put(r.reader)
	}
	return ent, err
}

// Index reads through a CAR file and returns the byte offsets of each block
// along with its CID, without loading the block data into memory.
func Index(reader io.Reader, opts ...IndexerOption) (HeaderIndexEntry, []BlockIndexEntry, error) {
	indexer, err := NewIndexer(reader, opts...)
	if err != nil {
		return HeaderIndexEntry{}, nil, err
	}

	var entries []BlockIndexEntry
	for {
		entry, err := indexer.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return HeaderIndexEntry{}, nil, err
		}
		entries = append(entries, entry)
	}

	return indexer.Header, entries, nil
}
