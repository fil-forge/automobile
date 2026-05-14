package automobile_test

import (
	"bytes"
	"encoding/hex"
	"io"
	"testing"

	"github.com/fil-forge/automobile"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

func TestIndex(t *testing.T) {
	blocks := []automobile.Block{
		randomBlock(t, 128),
		randomBlock(t, 1024),
		randomBlock(t, 4096),
	}
	roots := []cid.Cid{blocks[0].Link, blocks[2].Link}

	carBytes, err := io.ReadAll(automobile.Encode(roots, blocks))
	require.NoError(t, err)

	header, entries, err := automobile.Index(bytes.NewReader(carBytes))
	require.NoError(t, err)
	require.Equal(t, roots, header.Roots)
	require.Less(t, header.Size, uint64(len(carBytes)))

	require.Len(t, entries, len(blocks))
	for i, entry := range entries {
		require.True(t, entry.Link.Equals(blocks[i].Link))
		require.Equal(t, uint64(len(blocks[i].Data)-1), entry.End-entry.Start)
		require.Equal(t, blocks[i].Data, carBytes[entry.Start:entry.End+1])
	}

	// first block's data lives past the header
	require.Greater(t, entries[0].Start, header.Size)

	// offsets must be strictly increasing and non-overlapping
	for i := 1; i < len(entries); i++ {
		require.Greater(t, entries[i].Start, entries[i-1].End)
	}
}

func TestIndexSkipIntegrityChecks(t *testing.T) {
	blk := randomBlock(t, 64)
	carBytes, err := io.ReadAll(automobile.Encode([]cid.Cid{blk.Link}, []automobile.Block{blk}))
	require.NoError(t, err)

	// corrupt the block payload
	carBytes[len(carBytes)-1] ^= 0xff

	// default: integrity check fails
	_, _, err = automobile.Index(bytes.NewReader(carBytes))
	require.Error(t, err)

	// with skip: indexing succeeds
	_, _, err = automobile.Index(bytes.NewReader(carBytes), automobile.WithIndexerSkipIntegrityChecks())
	require.NoError(t, err)
}

func BenchmarkIndex_small(b *testing.B) {
	b.ReportAllocs()

	fixture, err := hex.DecodeString(fixtureHex)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		_, _, err := automobile.Index(bytes.NewReader(fixture))
		if err != nil {
			b.Fatal(err)
		}
	}
}
