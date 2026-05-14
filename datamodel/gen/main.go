//go:generate go run .

package main

import (
	"github.com/fil-forge/automobile/datamodel"
	cbg "github.com/whyrusleeping/cbor-gen"
)

func main() {
	if err := cbg.WriteMapEncodersToFile("../cbor_gen.go", "datamodel", datamodel.HeaderModel{}); err != nil {
		panic(err)
	}
}
