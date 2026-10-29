package pngembed_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/luchzin/png-embed"
)

func createTestImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8((x * 7) % 256),
				G: uint8((y * 13) % 256),
				B: uint8((x + y) % 256),
				A: 255,
			})
		}
	}
	return img
}

func TestEmbedAndExtractPixels(t *testing.T) {
	origImg := createTestImage(64, 64)
	secret := []byte("CTF{hidden_pixel_stego_flag_2026}")

	// 1. Embed secret in pixels
	stegoImg, err := pngembed.EmbedInPixels(origImg, secret)
	if err != nil {
		t.Fatalf("EmbedInPixels failed: %v", err)
	}

	// 2. Encode to PNG format (lossless compression)
	var buf bytes.Buffer
	if err := png.Encode(&buf, stegoImg); err != nil {
		t.Fatalf("png.Encode failed: %v", err)
	}

	// 3. Decode from PNG format
	decodedImg, err := png.Decode(&buf)
	if err != nil {
		t.Fatalf("png.Decode failed: %v", err)
	}

	// 4. Extract secret
	extracted, err := pngembed.ExtractFromPixels(decodedImg)
	if err != nil {
		t.Fatalf("ExtractFromPixels failed: %v", err)
	}

	if !bytes.Equal(extracted, secret) {
		t.Fatalf("mismatch: got %q, want %q", string(extracted), string(secret))
	}
}

func TestExtractNoSecret(t *testing.T) {
	origImg := createTestImage(32, 32)
	_, err := pngembed.ExtractFromPixels(origImg)
	if err != pngembed.ErrNoMagicFound {
		t.Fatalf("expected ErrNoMagicFound, got %v", err)
	}
}

func TestPayloadTooLarge(t *testing.T) {
	tinyImg := createTestImage(4, 4) // 16 pixels * 3 bits = 48 bits (header takes 64 bits)
	_, err := pngembed.EmbedInPixels(tinyImg, []byte("too long"))
	if err == nil {
		t.Fatal("expected error for payload exceeding capacity")
	}
}
