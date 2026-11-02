package pngembed_test

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/luchzin/png-embed"
)

func generateMinimalPNG() ([]byte, error) {
	img := createTestImage(16, 16)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestEmbedAndExtractCustomChunk(t *testing.T) {
	pngData, err := generateMinimalPNG()
	if err != nil {
		t.Fatalf("failed to generate PNG: %v", err)
	}

	customType := "cTFg"
	payload := []byte("CTF{hidden_in_custom_ancillary_chunk_2026}")

	// Embed chunk
	modifiedPNG, err := pngembed.EmbedChunk(pngData, customType, payload)
	if err != nil {
		t.Fatalf("EmbedChunk failed: %v", err)
	}

	// Verify the modified PNG is still 100% valid for standard image viewers/decoders
	_, err = png.Decode(bytes.NewReader(modifiedPNG))
	if err != nil {
		t.Fatalf("standard png.Decode failed on stego PNG: %v", err)
	}

	// Extract chunk
	extracted, err := pngembed.ExtractChunks(modifiedPNG, customType)
	if err != nil {
		t.Fatalf("ExtractChunks failed: %v", err)
	}
	if len(extracted) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(extracted))
	}
	if !bytes.Equal(extracted[0], payload) {
		t.Fatalf("payload mismatch: got %q, want %q", string(extracted[0]), string(payload))
	}
}

func TestEmbedAndExtractTextChunk(t *testing.T) {
	pngData, err := generateMinimalPNG()
	if err != nil {
		t.Fatalf("failed to generate PNG: %v", err)
	}

	keyword := "Comment"
	flagText := "CTF{standard_tEXt_chunk_flag}"

	stegoPNG, err := pngembed.EmbedTextChunk(pngData, keyword, flagText)
	if err != nil {
		t.Fatalf("EmbedTextChunk failed: %v", err)
	}

	// Still decodable
	if _, err := png.Decode(bytes.NewReader(stegoPNG)); err != nil {
		t.Fatalf("standard decoder failed: %v", err)
	}

	// Extract
	texts, err := pngembed.ExtractTextChunks(stegoPNG)
	if err != nil {
		t.Fatalf("ExtractTextChunks failed: %v", err)
	}

	if val, ok := texts[keyword]; !ok || val != flagText {
		t.Fatalf("expected text %q for keyword %q, got %q (all: %v)", flagText, keyword, val, texts)
	}
}

func TestListChunks(t *testing.T) {
	pngData, err := generateMinimalPNG()
	if err != nil {
		t.Fatalf("failed to generate PNG: %v", err)
	}

	stegoPNG, err := pngembed.EmbedChunk(pngData, "seCr", []byte("secret"))
	if err != nil {
		t.Fatalf("EmbedChunk failed: %v", err)
	}

	chunkInfos, err := pngembed.ListChunks(stegoPNG)
	if err != nil {
		t.Fatalf("ListChunks failed: %v", err)
	}

	foundSeCr := false
	for _, ci := range chunkInfos {
		if !ci.ValidCRC {
			t.Errorf("chunk %s at offset %d has invalid CRC", ci.Type, ci.Offset)
		}
		if ci.Type == "seCr" {
			foundSeCr = true
			if !ci.IsAncillary {
				t.Errorf("expected seCr to be marked ancillary")
			}
			if !ci.IsPrivate {
				t.Errorf("expected seCr to be marked private")
			}
		}
	}

	if !foundSeCr {
		t.Fatal("custom chunk seCr not found in ListChunks output")
	}
}

func TestValidateChunkType(t *testing.T) {
	valid := []string{"tEXt", "cTFg", "IHDR", "IDAT", "IEND", "flag"}
	for _, v := range valid {
		if err := pngembed.ValidateChunkType(v); err != nil {
			t.Errorf("expected %q to be valid, got %v", v, err)
		}
	}

	invalid := []string{"1234", "toolong", "sh", "tE!t", ""}
	for _, inv := range invalid {
		if err := pngembed.ValidateChunkType(inv); err == nil {
			t.Errorf("expected %q to be invalid, got nil", inv)
		}
	}
}

