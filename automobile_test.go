package automobile_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"strings"
	"testing"

	"github.com/fil-forge/automobile"
	"github.com/ipfs/go-cid"
	multihash "github.com/multiformats/go-multihash/core"
	"github.com/stretchr/testify/require"
)

// fixtureHex is a clean single-block, single-root CAR
const fixtureHex = "3aa265726f6f747381d82a58250001711220151fe9e73c6267a7060c6f6c4cca943c236f4b196723489608edb42a8b8fa80b6776657273696f6e012c01711220151fe9e73c6267a7060c6f6c4cca943c236f4b196723489608edb42a8b8fa80ba165646f646779f5"

func randomBytes(t *testing.T, size int) []byte {
	t.Helper()
	bytes := make([]byte, size)
	_, err := rand.Read(bytes)
	require.NoError(t, err)
	return bytes
}

func randomBlock(t *testing.T, size int) automobile.Block {
	t.Helper()
	b := randomBytes(t, size)
	c, err := cid.V1Builder{Codec: cid.Raw, MhType: multihash.SHA2_256}.Sum(b)
	require.NoError(t, err)
	return automobile.Block{Link: c, Data: b}
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	blocks := []automobile.Block{
		randomBlock(t, 128),
		randomBlock(t, 1024),
		randomBlock(t, 4096),
	}
	roots := []cid.Cid{blocks[0].Link, blocks[2].Link}

	carBytes, err := io.ReadAll(automobile.Encode(roots, blocks))
	require.NoError(t, err)

	gotRoots, gotBlocks, err := automobile.Decode(bytes.NewReader(carBytes))
	require.NoError(t, err)
	require.Equal(t, roots, gotRoots)

	require.Len(t, gotBlocks, len(blocks))
	for i, blk := range gotBlocks {
		require.True(t, blk.Link.Equals(blocks[i].Link))
		require.Equal(t, blocks[i].Data, blk.Data)
	}
}

func TestEncodeDecodeNoBlocks(t *testing.T) {
	roots := []cid.Cid{}

	r := automobile.Encode(roots, nil)
	defer r.Close()

	gotRoots, gotBlocks, err := automobile.Decode(r)
	require.NoError(t, err)
	require.Empty(t, gotRoots)
	require.Empty(t, gotBlocks)
}

func TestDecodeRejectsGarbage(t *testing.T) {
	_, _, err := automobile.Decode(bytes.NewReader([]byte("not a CAR file")))
	require.Error(t, err)
}

func TestDecodeRejectsCorruptBlock(t *testing.T) {
	blk := randomBlock(t, 64)
	carBytes, err := io.ReadAll(automobile.Encode([]cid.Cid{blk.Link}, []automobile.Block{blk}))
	require.NoError(t, err)

	// flip a byte in the block payload so the hash no longer matches the CID
	carBytes[len(carBytes)-1] ^= 0xff

	_, _, err = automobile.Decode(bytes.NewReader(carBytes))
	require.Error(t, err)
}

// Lifted from https://github.com/ipld/go-car/blob/c4b9f366f20cec0f90da0dd02134e42955a6e217/car_test.go#L81-L141
func TestEOFHandling(t *testing.T) {
	fixture, err := hex.DecodeString(fixtureHex)
	require.NoError(t, err)

	t.Run("CleanEOF", func(t *testing.T) {
		roots, blocks, err := automobile.Decode(bytes.NewReader(fixture))
		require.NoError(t, err)
		require.Len(t, roots, 1)
		require.Equal(t, "bafyreiavd7u6opdcm6tqmddpnrgmvfb4enxuwglhenejmchnwqvixd5ibm", roots[0].String())
		require.Len(t, blocks, 1)
		require.Equal(t, "bafyreiavd7u6opdcm6tqmddpnrgmvfb4enxuwglhenejmchnwqvixd5ibm", blocks[0].Link.String())
	})

	t.Run("BadVarint", func(t *testing.T) {
		_, _, err := automobile.Decode(bytes.NewReader(append(fixture, 160)))
		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("TruncatedBlock", func(t *testing.T) {
		_, _, err := automobile.Decode(bytes.NewReader(append(fixture, 100, 0, 0)))
		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})
}

// Lifted from https://github.com/ipld/go-car/blob/c4b9f366f20cec0f90da0dd02134e42955a6e217/car_test.go#L189-L275
func TestBadHeaders(t *testing.T) {
	testCases := []struct {
		name   string
		hex    string
		errStr string // either the whole error string
		errPfx string // or just the prefix
	}{
		{
			"{version:2}",
			"0aa16776657273696f6e02",
			"invalid car version: 2",
			"",
		},
		{
			// an unfortunate error because we don't use a pointer
			"{roots:[baeaaaa3bmjrq]}",
			"13a165726f6f747381d82a480001000003616263",
			"invalid car version: 0",
			"",
		},
		{
			"{version:\"1\",roots:[baeaaaa3bmjrq]}",
			"1da265726f6f747381d82a4800010000036162636776657273696f6e6131",
			"", "invalid header: ",
		},
		// FIXME: cbor-gen does not care about missing fields
		// {
		// 	"{version:1}",
		// 	"0aa16776657273696f6e01",
		// 	"empty car, no roots",
		// 	"",
		// },
		{
			"{version:1,roots:{cid:baeaaaa3bmjrq}}",
			"20a265726f6f7473a163636964d82a4800010000036162636776657273696f6e01",
			"",
			"invalid header: ",
		},
		// FIXME: cbor-gen does not care about additional fields
		// {
		// 	"{version:1,roots:[baeaaaa3bmjrq],blip:true}",
		// 	"22a364626c6970f565726f6f747381d82a4800010000036162636776657273696f6e01",
		// 	"",
		// 	"invalid header: ",
		// },
		{
			"[1,[]]",
			"03820180",
			"",
			"invalid header: ",
		},
		{
			// this is an unfortunate error, it'd be nice to catch it better but it's
			// very unlikely we'd ever see this in practice
			"null",
			"01f6",
			"",
			"invalid header: cbor input should be of type map",
		},
	}

	makeCar := func(t *testing.T, byts string) error {
		fixture, err := hex.DecodeString(byts)
		require.NoError(t, err)
		_, _, err = automobile.Decode(bytes.NewReader(fixture))
		return err
	}

	t.Run("Sanity check {version:1,roots:[baeaaaa3bmjrq]}", func(t *testing.T) {
		err := makeCar(t, "1ca265726f6f747381d82a4800010000036162636776657273696f6e01")
		require.NoError(t, err)
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := makeCar(t, tc.hex)
			require.Error(t, err, "expected error from bad header, didn't get one")
			if tc.errStr != "" {
				require.Equal(t, tc.errStr, err.Error())
			} else {
				require.True(t, strings.HasPrefix(err.Error(), tc.errPfx), "error did not have expected prefix %q: %v", tc.errPfx, err)
			}
		})
	}
}

func BenchmarkDecode_small(b *testing.B) {
	b.ReportAllocs()

	fixture, err := hex.DecodeString(fixtureHex)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		_, _, err := automobile.Decode(bytes.NewReader(fixture))
		if err != nil {
			b.Fatal(err)
		}
	}
}
