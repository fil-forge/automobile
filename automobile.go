package automobile

import (
	"bufio"
	"fmt"
	"io"

	"github.com/fil-forge/automobile/datamodel"
	"github.com/ipfs/go-cid"
)

// Code is the multicodec code for CAR files.
// See https://github.com/multiformats/multicodec/blob/45c88b89ab909c0fac7c86dafe43ad72d1e8e8a9/table.csv#L143
const Code = 0x0202

// ContentType is the value the HTTP Content-Type header should have for CARs.
// See https://www.iana.org/assignments/media-types/application/vnd.ipld.car
const ContentType = "application/vnd.ipld.car"

type Block struct {
	Link cid.Cid
	Data []byte
}

type Writer struct {
	writer        io.Writer
	headerWritten bool
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{writer: w}
}

func (e *Writer) WriteHeader(roots []cid.Cid) error {
	if e.headerWritten {
		return fmt.Errorf("header already written")
	}
	err := WriteHeader(e.writer, roots)
	if err == nil {
		e.headerWritten = true
	}
	return err
}

func (e *Writer) WriteBlock(block Block) error {
	if !e.headerWritten {
		err := e.WriteHeader(nil)
		if err != nil {
			return err
		}
	}
	return WriteBlock(e.writer, block)
}

// Encode takes a list of root CIDs and blocks, and produces an io.ReadCloser
// that emits the bytes of a CAR file containing those roots and blocks. The
// caller should call Close on the returned ReadCloser when finished to free up
// resources.
func Encode(roots []cid.Cid, blocks []Block) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		carWriter := NewWriter(writer)
		if err := carWriter.WriteHeader(roots); err != nil {
			writer.CloseWithError(fmt.Errorf("writing CAR header: %w", err))
			return
		}
		for _, block := range blocks {
			err := carWriter.WriteBlock(block)
			if err != nil {
				writer.CloseWithError(fmt.Errorf("writing CAR block %q: %w", block.Link, err))
				return
			}
		}
		writer.Close()
	}()
	return reader
}

type readerCfg struct {
	skipIntegrityChecks bool
}

type ReaderOption func(*readerCfg)

// WithSkipIntegrityChecks configures the reader to skip integrity
// checks when reading blocks. This can be used to speed up decoding when the
// integrity of the blocks is not a concern.
func WithSkipIntegrityChecks() ReaderOption {
	return func(cfg *readerCfg) {
		cfg.skipIntegrityChecks = true
	}
}

type Reader struct {
	Header      datamodel.HeaderModel
	reader      *bufio.Reader
	blockReader *BlockReader
}

func NewReader(r io.Reader, opts ...ReaderOption) (*Reader, error) {
	cfg := &readerCfg{}
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
		return nil, fmt.Errorf("invalid car version: %d", h.Version)
	}

	blr := BlockReader{Reader: br, SkipIntegrityChecks: cfg.skipIntegrityChecks}
	return &Reader{Header: h, reader: br, blockReader: &blr}, nil
}

func (r *Reader) Read() (Block, error) {
	b, err := r.blockReader.Read()
	if err == io.EOF {
		// Common happy case: recycle the bufio.Reader.
		// In the other error paths leaking it is fine.
		bufioReaderPool.Put(r.reader)
	}
	return b, err
}

// Decode takes an io.Reader containing the bytes of a CAR file and returns the
// list of root CIDs along with all the blocks in the CAR.
func Decode(reader io.Reader, opts ...ReaderOption) ([]cid.Cid, []Block, error) {
	carReader, err := NewReader(reader, opts...)
	if err != nil {
		return nil, nil, err
	}

	var blocks []Block
	for {
		blk, err := carReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		blocks = append(blocks, blk)
	}
	return carReader.Header.Roots, blocks, nil
}
