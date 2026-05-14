package automobile_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/fil-forge/automobile"
	"github.com/ipfs/go-cid"
)

func TestEncode(t *testing.T) {
	blocks := []automobile.Block{
		randomBlock(t, 360),
		randomBlock(t, 720),
		randomBlock(t, 1080),
	}
	roots := []cid.Cid{blocks[len(blocks)-1].Link}

	carBytes, err := io.ReadAll(automobile.Encode(roots, blocks))
	if err != nil {
		panic(err)
	}

	fmt.Printf("Wrote %d byte CAR\n", len(carBytes))
}

func TestWriter(t *testing.T) {
	var carBuf bytes.Buffer
	writer := automobile.NewWriter(&carBuf)

	// If you know the root(s), write them first.
	//
	// If you do not know the root(s), you can omit this call and the writer will
	// write a header with an empty roots automatically, on the first call to
	// WriteBlock.
	err := writer.WriteHeader([]cid.Cid{})
	if err != nil {
		panic(err)
	}

	for range 3 {
		blk := randomBlock(t, 360)
		if err := writer.WriteBlock(blk); err != nil {
			panic(err)
		}
	}

	fmt.Printf("Wrote %d byte CAR\n", carBuf.Len())
}

func TestDecode(t *testing.T) {
	f, err := os.Open("./testdata/fixtures/comic.car")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	roots, blocks, err := automobile.Decode(f)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Roots: %v\n", roots)
	for _, block := range blocks {
		fmt.Printf("Block: %s (%d bytes)\n", block.Link, len(block.Data))
	}
}

func TestReader(t *testing.T) {
	f, err := os.Open("./testdata/fixtures/comic.car")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	reader, err := automobile.NewReader(f)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Roots: %v\n", reader.Header.Roots)

	for {
		block, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		fmt.Printf("Block: %s (%d bytes)\n", block.Link, len(block.Data))
	}
}
