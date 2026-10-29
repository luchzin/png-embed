package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image/png"
	"os"

	"github.com/luchzin/png-embed"
)

var pngSig = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

type Chunk struct {
	Type string
	Data []byte
}

func ParsePNG(b []byte) ([]Chunk, error) {
	if len(b) < 8 || !bytes.Equal(b[:8], pngSig) {
		return nil, errors.New("bad PNG signature")
	}
	var chunks []Chunk
	i := 8
	for i+12 <= len(b) {
		length := int(binary.BigEndian.Uint32(b[i : i+4]))
		if i+12+length > len(b) {
			return nil, fmt.Errorf("chunk at %d overruns file", i)
		}
		typ := string(b[i+4 : i+8])
		data := b[i+8 : i+8+length]
		crc := binary.BigEndian.Uint32(b[i+8+length : i+12+length])
		calc := crc32.ChecksumIEEE(b[i+4 : i+8+length])
		if crc != calc {
			fmt.Printf("warn: bad CRC in %s (fixing on write)\n", typ)
		}
		chunks = append(chunks, Chunk{typ, append([]byte(nil), data...)})
		i += 12 + length
		if typ == "IEND" {
			if i < len(b) {
				fmt.Printf("warn: %d trailing bytes after IEND dropped\n", len(b)-i)
			}
			break
		}
	}
	return chunks, nil
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  go run ./examples/basic embed <in.png> <out.png> <key/flag>")
	fmt.Println("  go run ./examples/basic extract <in.png>")
	fmt.Println("  go run ./examples/basic chunks <in.png>")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "embed":
		if len(os.Args) < 5 {
			fmt.Println("Usage: embed <in.png> <out.png> <secret>")
			os.Exit(1)
		}
		inPath := os.Args[2]
		outPath := os.Args[3]
		secret := []byte(os.Args[4])

		inFile, err := os.Open(inPath)
		if err != nil {
			panic(err)
		}
		defer inFile.Close()

		img, err := png.Decode(inFile)
		if err != nil {
			panic(fmt.Errorf("failed to decode PNG: %w", err))
		}

		stegoImg, err := pngembed.EmbedInPixels(img, secret)
		if err != nil {
			panic(fmt.Errorf("embed failed: %w", err))
		}

		outFile, err := os.Create(outPath)
		if err != nil {
			panic(err)
		}
		defer outFile.Close()

		if err := png.Encode(outFile, stegoImg); err != nil {
			panic(fmt.Errorf("failed to encode PNG: %w", err))
		}

		fmt.Printf("Successfully embedded %d bytes into %s\n", len(secret), outPath)

	case "extract":
		if len(os.Args) < 3 {
			fmt.Println("Usage: extract <in.png>")
			os.Exit(1)
		}
		inPath := os.Args[2]

		inFile, err := os.Open(inPath)
		if err != nil {
			panic(err)
		}
		defer inFile.Close()

		img, err := png.Decode(inFile)
		if err != nil {
			panic(fmt.Errorf("failed to decode PNG: %w", err))
		}

		extracted, err := pngembed.ExtractFromPixels(img)
		if err != nil {
			panic(fmt.Errorf("extract failed: %w", err))
		}

		fmt.Printf("Extracted Secret: %s\n", string(extracted))

	case "chunks":
		if len(os.Args) < 3 {
			fmt.Println("Usage: chunks <in.png>")
			os.Exit(1)
		}
		in, err := os.ReadFile(os.Args[2])
		if err != nil {
			panic(err)
		}
		chunks, err := ParsePNG(in)
		if err != nil {
			panic(err)
		}
		for _, c := range chunks {
			fmt.Println(c.Type, len(c.Data))
		}

	default:
		printUsage()
		os.Exit(1)
	}
}
