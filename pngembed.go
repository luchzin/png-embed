package pngembed

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
)

var (
	// MagicHeader is prepended before the data to identify embedded messages.
	MagicHeader = []byte("STEG")

	ErrPayloadTooLarge = errors.New("payload exceeds maximum image pixel capacity")
	ErrNoMagicFound    = errors.New("no embedded data found (magic header mismatch)")
	ErrCorruptData     = errors.New("embedded data length corrupted or out of bounds")
)

// ToNRGBA converts any image.Image into a mutable *image.NRGBA.
func ToNRGBA(src image.Image) *image.NRGBA {
	bounds := src.Bounds()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)
	return dst
}

// MaxPixelCapacity returns the maximum number of payload bytes that can fit in the image.
func MaxPixelCapacity(bounds image.Rectangle) int {
	// Each pixel has 3 color channels (R, G, B) = 3 bits per pixel.
	totalPixels := bounds.Dx() * bounds.Dy()
	totalBits := totalPixels * 3
	headerBits := (len(MagicHeader) + 4) * 8 // magic + 4 bytes length
	if totalBits <= headerBits {
		return 0
	}
	return (totalBits - headerBits) / 8
}

// EmbedInPixels embeds secret payload bytes into the least significant bits
// of the RGB channels of src image.
func EmbedInPixels(src image.Image, payload []byte) (*image.NRGBA, error) {
	bounds := src.Bounds()
	maxCapacity := MaxPixelCapacity(bounds)
	if len(payload) > maxCapacity {
		return nil, fmt.Errorf("%w: capacity is %d bytes, payload is %d bytes",
			ErrPayloadTooLarge, maxCapacity, len(payload))
	}

	// Construct bitstream: Magic (4 bytes) + Length (4 bytes uint32) + Payload
	var buf []byte
	buf = append(buf, MagicHeader...)
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, uint32(len(payload)))
	buf = append(buf, lenBytes...)
	buf = append(buf, payload...)

	img := ToNRGBA(src)
	bitIdx := 0
	totalBits := len(buf) * 8

	for y := bounds.Min.Y; y < bounds.Max.Y && bitIdx < totalBits; y++ {
		for x := bounds.Min.X; x < bounds.Max.X && bitIdx < totalBits; x++ {
			c := img.NRGBAAt(x, y)
			channels := []*uint8{&c.R, &c.G, &c.B}

			for _, ch := range channels {
				if bitIdx >= totalBits {
					break
				}
				// Get current bit from buffer
				byteVal := buf[bitIdx/8]
				bit := (byteVal >> (7 - (bitIdx % 8))) & 1

				// Clear lowest bit and set to our bit
				*ch = (*ch & 0xFE) | bit
				bitIdx++
			}

			img.SetNRGBA(x, y, c)
		}
	}

	return img, nil
}

// ExtractFromPixels extracts the embedded secret payload from the RGB channels.
func ExtractFromPixels(src image.Image) ([]byte, error) {
	bounds := src.Bounds()
	img := ToNRGBA(src)

	// Helper iterator to read bits sequentially
	type bitReader struct {
		x, y    int
		channel int
	}

	r := &bitReader{x: bounds.Min.X, y: bounds.Min.Y, channel: 0}
	readBit := func() (byte, bool) {
		if r.y >= bounds.Max.Y {
			return 0, false
		}
		c := img.NRGBAAt(r.x, r.y)
		var val uint8
		switch r.channel {
		case 0:
			val = c.R
		case 1:
			val = c.G
		case 2:
			val = c.B
		}

		bit := val & 1
		r.channel++
		if r.channel == 3 {
			r.channel = 0
			r.x++
			if r.x >= bounds.Max.X {
				r.x = bounds.Min.X
				r.y++
			}
		}
		return bit, true
	}

	readBytes := func(n int) ([]byte, error) {
		out := make([]byte, n)
		for i := 0; i < n; i++ {
			var b byte
			for bit := 0; bit < 8; bit++ {
				bitVal, ok := readBit()
				if !ok {
					return nil, errors.New("unexpected end of pixel stream")
				}
				b = (b << 1) | bitVal
			}
			out[i] = b
		}
		return out, nil
	}

	// 1. Read and check magic header
	magic, err := readBytes(len(MagicHeader))
	if err != nil || string(magic) != string(MagicHeader) {
		return nil, ErrNoMagicFound
	}

	// 2. Read length prefix
	lenBytes, err := readBytes(4)
	if err != nil {
		return nil, ErrCorruptData
	}
	payloadLen := int(binary.BigEndian.Uint32(lenBytes))
	if payloadLen < 0 || payloadLen > MaxPixelCapacity(bounds) {
		return nil, ErrCorruptData
	}

	// 3. Read payload
	payload, err := readBytes(payloadLen)
	if err != nil {
		return nil, ErrCorruptData
	}

	return payload, nil
}
