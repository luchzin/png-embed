package pngembed

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
)

var (
	MagicHeader        = []byte("STEG")
	ErrPayloadTooLarge = errors.New("payload exceeds maximum image pixel capacity")
	ErrNoMagicFound    = errors.New("no embedded data found (magic header mismatch)")
	ErrCorruptData     = errors.New("embedded data length corrupted or out of bounds")
)

func ToNRGBA(src image.Image) *image.NRGBA {
	bounds := src.Bounds()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)
	return dst
}

func MaxPixelCapacity(bounds image.Rectangle) int {
	totalPixels := bounds.Dx() * bounds.Dy()
	totalBits := totalPixels * 3
	headerBits := (len(MagicHeader) + 4) * 8
	if totalBits <= headerBits {
		return 0
	}
	return (totalBits - headerBits) / 8
}

func EmbedInPixels(src image.Image, payload []byte) (*image.NRGBA, error) {
	bounds := src.Bounds()
	maxCapacity := MaxPixelCapacity(bounds)
	if len(payload) > maxCapacity {
		return nil, fmt.Errorf("%w: capacity is %d bytes, payload is %d bytes",
			ErrPayloadTooLarge, maxCapacity, len(payload))
	}

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
				byteVal := buf[bitIdx/8]
				bit := (byteVal >> (7 - (bitIdx % 8))) & 1
				*ch = (*ch & 0xFE) | bit
				bitIdx++
			}

			img.SetNRGBA(x, y, c)
		}
	}

	return img, nil
}

func ExtractFromPixels(src image.Image) ([]byte, error) {
	bounds := src.Bounds()
	img := ToNRGBA(src)

	type bitReader struct {
		x, y    int
		channel int
	}

	r := &bitReader{x: bounds.Min.X, y: bounds.Min.Y}
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

	magic, err := readBytes(len(MagicHeader))
	if err != nil || string(magic) != string(MagicHeader) {
		return nil, ErrNoMagicFound
	}

	lenBytes, err := readBytes(4)
	if err != nil {
		return nil, ErrCorruptData
	}
	payloadLen := int(binary.BigEndian.Uint32(lenBytes))
	if payloadLen < 0 || payloadLen > MaxPixelCapacity(bounds) {
		return nil, ErrCorruptData
	}

	payload, err := readBytes(payloadLen)
	if err != nil {
		return nil, ErrCorruptData
	}

	return payload, nil
}
