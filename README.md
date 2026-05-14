# automobile

Bare bones CAR file encoder and decoder. Depends only on `go-cid`.

## Install

```sh
go get github.com/fil-forge/automobile
```

## Usage

See [examples](./examples_test.go).

### Encode

```go
roots := []cid.Cid{ /* ... */ }
blocks := []automobile.Block{ /* ... */ }

reader, err := automobile.Encode(roots, blocks)

io.ReadAll(reader)
```

### Streaming encode

```go
var b bytes.Buffer
writer := NewWriter(&b)

// If you know the root(s), write them first.
//
// If you do not know the root(s), you can omit this call and the writer will
// write a header with an empty roots automatically, on the first call to
// WriteBlock.
err := writer.WriteHeader([]cid.Cid{ /* ... */ })

// repeat until all blocks written
err := writer.WriteBlock(automobile.Block{ /* ... */ })
```

### Decode

```go
f, err := os.Open("./path/to/my.car")

roots, blocks, err := automobile.Decode(f)

fmt.Printf("Roots: %v\n", roots)
for _, block := range blocks {
  fmt.Printf("Block: %s (%d bytes)\n", block.Link, len(block.Data))
}
```

### Streaming decode

```go
f, err := os.Open("./testdata/fixtures/comic.car")

reader, err := automobile.NewReader(f)

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
```

## Contributing

Feel free to join in. All welcome. Please [open an issue](https://github.com/fil-forge/automobile/issues)!

## License

Dual-licensed under [MIT OR Apache 2.0](LICENSE.md)