package main

import (
	"fmt"
	"image/png"
	"os"

	"github.com/luchzin/png-embed"
)

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  Pixel LSB Steganography:")
	fmt.Println("    go run ./examples/basic embed <in.png> <out.png> <secret>")
	fmt.Println("    go run ./examples/basic extract <in.png>")
	fmt.Println("")
	fmt.Println("  Custom Ancillary PNG Chunks:")
	fmt.Println("    go run ./examples/basic embed-chunk <in.png> <out.png> <type> <secret>")
	fmt.Println("    go run ./examples/basic extract-chunk <in.png> <type>")
	fmt.Println("    go run ./examples/basic embed-text <in.png> <out.png> <keyword> <text>")
	fmt.Println("    go run ./examples/basic extract-text <in.png>")
	fmt.Println("    go run ./examples/basic list-chunks <in.png>")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	// Pixel LSB Steganography
	case "embed", "embed-pixel":
		if len(os.Args) < 5 {
			fmt.Println("Usage: embed <in.png> <out.png> <secret>")
			os.Exit(1)
		}
		inPath, outPath, secret := os.Args[2], os.Args[3], []byte(os.Args[4])

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

		fmt.Printf("Successfully embedded %d bytes into pixels of %s\n", len(secret), outPath)

	case "extract", "extract-pixel":
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

		fmt.Printf("Extracted Secret from pixels: %s\n", string(extracted))

	// Custom Chunk Steganography
	case "embed-chunk":
		if len(os.Args) < 6 {
			fmt.Println("Usage: embed-chunk <in.png> <out.png> <4-char-type> <secret>")
			os.Exit(1)
		}
		inPath, outPath, chunkType, secret := os.Args[2], os.Args[3], os.Args[4], []byte(os.Args[5])

		data, err := os.ReadFile(inPath)
		if err != nil {
			panic(err)
		}

		outData, err := pngembed.EmbedChunk(data, chunkType, secret)
		if err != nil {
			panic(fmt.Errorf("embed-chunk failed: %w", err))
		}

		if err := os.WriteFile(outPath, outData, 0o644); err != nil {
			panic(err)
		}

		fmt.Printf("Successfully injected chunk %q (%d bytes) into %s\n", chunkType, len(secret), outPath)

	case "extract-chunk":
		if len(os.Args) < 4 {
			fmt.Println("Usage: extract-chunk <in.png> <4-char-type>")
			os.Exit(1)
		}
		inPath, chunkType := os.Args[2], os.Args[3]

		data, err := os.ReadFile(inPath)
		if err != nil {
			panic(err)
		}

		chunks, err := pngembed.ExtractChunks(data, chunkType)
		if err != nil {
			panic(fmt.Errorf("extract-chunk failed: %w", err))
		}

		for idx, c := range chunks {
			fmt.Printf("[%d] %s: %s\n", idx, chunkType, string(c))
		}

	case "embed-text":
		if len(os.Args) < 6 {
			fmt.Println("Usage: embed-text <in.png> <out.png> <keyword> <text>")
			os.Exit(1)
		}
		inPath, outPath, keyword, text := os.Args[2], os.Args[3], os.Args[4], os.Args[5]

		data, err := os.ReadFile(inPath)
		if err != nil {
			panic(err)
		}

		outData, err := pngembed.EmbedTextChunk(data, keyword, text)
		if err != nil {
			panic(fmt.Errorf("embed-text failed: %w", err))
		}

		if err := os.WriteFile(outPath, outData, 0o644); err != nil {
			panic(err)
		}

		fmt.Printf("Successfully added tEXt chunk (%s: %s) to %s\n", keyword, text, outPath)

	case "extract-text":
		if len(os.Args) < 3 {
			fmt.Println("Usage: extract-text <in.png>")
			os.Exit(1)
		}
		inPath := os.Args[2]

		data, err := os.ReadFile(inPath)
		if err != nil {
			panic(err)
		}

		texts, err := pngembed.ExtractTextChunks(data)
		if err != nil {
			panic(fmt.Errorf("extract-text failed: %w", err))
		}

		for k, v := range texts {
			fmt.Printf("%s: %s\n", k, v)
		}

	case "chunks", "list-chunks":
		if len(os.Args) < 3 {
			fmt.Println("Usage: list-chunks <in.png>")
			os.Exit(1)
		}
		inPath := os.Args[2]

		data, err := os.ReadFile(inPath)
		if err != nil {
			panic(err)
		}

		chunks, err := pngembed.ListChunks(data)
		if err != nil {
			panic(fmt.Errorf("list-chunks failed: %w", err))
		}

		fmt.Printf("%-8s %-10s %-12s %-10s %-10s\n", "Type", "Length", "Offset", "Ancillary?", "Valid CRC?")
		fmt.Println("---------------------------------------------------------")
		for _, c := range chunks {
			fmt.Printf("%-8s %-10d 0x%-10X %-10v %-10v\n",
				c.Type, c.Length, c.Offset, c.IsAncillary, c.ValidCRC)
		}

	default:
		printUsage()
		os.Exit(1)
	}
}
