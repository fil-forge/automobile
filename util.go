package automobile

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/fil-forge/automobile/datamodel"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-varint"
)

// MaxAllowedSectionSize dictates the maximum number of bytes that a CARv1
// header or block is allowed to occupy without causing a decode to error.
var MaxAllowedSectionSize uint = 32 << 20 // 32MiB

var bufioReaderPool = sync.Pool{
	New: func() any { return bufio.NewReader(nil) },
}

func WriteHeader(w io.Writer, roots []cid.Cid) error {
	h := datamodel.HeaderModel{Roots: roots, Version: 1}
	var hb bytes.Buffer
	if err := h.MarshalCBOR(&hb); err != nil {
		return fmt.Errorf("writing CAR header: %w", err)
	}
	return LdWrite(w, hb.Bytes())
}

func ReadHeader(br *bufio.Reader) (datamodel.HeaderModel, error) {
	hb, err := LdRead(br)
	if err != nil {
		return datamodel.HeaderModel{}, err
	}
	var h datamodel.HeaderModel
	if err := h.UnmarshalCBOR(bytes.NewReader(hb)); err != nil {
		return datamodel.HeaderModel{}, fmt.Errorf("invalid header: %w", err)
	}
	return h, nil
}

func WriteBlock(w io.Writer, block Block) error {
	return LdWrite(w, block.Link.Bytes(), block.Data)
}

func ReadBlock(br *bufio.Reader) (cid.Cid, []byte, error) {
	data, err := LdRead(br)
	if err != nil {
		return cid.Cid{}, nil, err
	}

	n, c, err := cid.CidFromReader(bytes.NewReader(data))
	if err != nil {
		return cid.Cid{}, nil, err
	}

	return c, data[n:], nil
}

func HeaderSize(h datamodel.HeaderModel) (uint64, error) {
	var hb bytes.Buffer
	if err := h.MarshalCBOR(&hb); err != nil {
		return 0, fmt.Errorf("writing CAR header: %w", err)
	}
	return LdSize(hb.Bytes()), nil
}

func LdWrite(w io.Writer, d ...[]byte) error {
	var sum uint64
	for _, s := range d {
		sum += uint64(len(s))
	}

	buf := make([]byte, 8)
	n := binary.PutUvarint(buf, sum)
	_, err := w.Write(buf[:n])
	if err != nil {
		return err
	}

	for _, s := range d {
		_, err = w.Write(s)
		if err != nil {
			return err
		}
	}

	return nil
}

func LdSize(d ...[]byte) uint64 {
	var sum uint64
	for _, s := range d {
		sum += uint64(len(s))
	}
	buf := make([]byte, 8)
	n := binary.PutUvarint(buf, sum)
	return sum + uint64(n)
}

func LdRead(r *bufio.Reader) ([]byte, error) {
	if _, err := r.Peek(1); err != nil { // no more blocks, likely clean io.EOF
		return nil, err
	}

	l, err := binary.ReadUvarint(r)
	if err != nil {
		if err == io.EOF {
			return nil, io.ErrUnexpectedEOF // don't silently pretend this is a clean EOF
		}
		return nil, err
	}

	if l > uint64(MaxAllowedSectionSize) { // Don't OOM
		return nil, errors.New("malformed car; header is bigger than util.MaxAllowedSectionSize")
	}

	buf := make([]byte, l)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}

	return buf, nil
}

// BlockReader is a helper for iterating through blocks in a CAR file.
type BlockReader struct {
	Reader              *bufio.Reader
	SkipIntegrityChecks bool
}

func (r *BlockReader) Read() (Block, error) {
	cid, bytes, err := ReadBlock(r.Reader)
	if err != nil {
		return Block{}, err
	}
	if !r.SkipIntegrityChecks {
		hashed, err := cid.Prefix().Sum(bytes)
		if err != nil {
			return Block{}, err
		}
		if !hashed.Equals(cid) {
			return Block{}, fmt.Errorf("mismatch in content integrity, name: %s, data: %s", cid, hashed)
		}
	}
	return Block{Link: cid, Data: bytes}, nil
}

// BlockIndexer is a helper for iterating through blocks in a CAR file while
// keeping track of their byte offsets for indexing purposes.
type BlockIndexer struct {
	Reader              *bufio.Reader
	Offset              uint64
	SkipIntegrityChecks bool
}

func (bi *BlockIndexer) Read() (BlockIndexEntry, error) {
	cid, bytes, err := ReadBlock(bi.Reader)
	if err != nil {
		return BlockIndexEntry{}, err
	}

	if !bi.SkipIntegrityChecks {
		hashed, err := cid.Prefix().Sum(bytes)
		if err != nil {
			return BlockIndexEntry{}, err
		}

		if !hashed.Equals(cid) && !bi.SkipIntegrityChecks {
			return BlockIndexEntry{}, fmt.Errorf("mismatch in content integrity, name: %s, data: %s", cid, hashed)
		}
	}

	ss := uint64(cid.ByteLen()) + uint64(len(bytes))
	bi.Offset += uint64(varint.UvarintSize(ss)) + ss

	start := bi.Offset - uint64(len(bytes))
	end := start + uint64(len(bytes)) - 1
	return BlockIndexEntry{Link: cid, Start: start, End: end}, nil
}
