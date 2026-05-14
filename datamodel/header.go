package datamodel

import "github.com/ipfs/go-cid"

type HeaderModel struct {
	Roots   []cid.Cid `cborgen:"roots"`
	Version int64     `cborgen:"version"`
}
