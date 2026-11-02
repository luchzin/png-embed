package pngembed

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"unicode"
)

var (
	PNGSignature           = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	ErrInvalidPNGSignature = errors.New("invalid PNG signature")
	ErrCorruptChunk        = errors.New("corrupted PNG chunk or overrun")
	ErrInvalidChunkType    = errors.New("invalid chunk type: must be exactly 4 ASCII letters")
	ErrChunkNotFound       = errors.New("specified chunk not found")
	ErrMissingIEND         = errors.New("PNG does not contain an IEND chunk")
	ErrInvalidKeyword      = errors.New("tEXt keyword must be 1 to 79 printable ASCII characters")
)

type Chunk struct {
	Type string
	Data []byte
	CRC  uint32
}

type ChunkInfo struct {
	Type        string
	Length      int
	Offset      int64
	CRC         uint32
	ComputedCRC uint32
	ValidCRC    bool
	IsAncillary bool
	IsPrivate   bool
}

func ValidateChunkType(t string) error {
	if len(t) != 4 {
		return ErrInvalidChunkType
	}
	for _, r := range t {
		if !unicode.IsLetter(r) || r > 127 {
			return ErrInvalidChunkType
		}
	}
	return nil
}

func ParseChunks(b []byte) ([]Chunk, error) {
	if len(b) < 8 || !bytes.Equal(b[:8], PNGSignature) {
		return nil, ErrInvalidPNGSignature
	}

	var chunks []Chunk
	i := 8
	for i+12 <= len(b) {
		length := int(binary.BigEndian.Uint32(b[i : i+4]))
		if i+12+length > len(b) {
			return nil, fmt.Errorf("%w: chunk starting at offset %d overruns file", ErrCorruptChunk, i)
		}

		typ := string(b[i+4 : i+8])
		data := b[i+8 : i+8+length]
		storedCRC := binary.BigEndian.Uint32(b[i+8+length : i+12+length])

		dataCopy := make([]byte, length)
		copy(dataCopy, data)

		chunks = append(chunks, Chunk{
			Type: typ,
			Data: dataCopy,
			CRC:  storedCRC,
		})

		i += 12 + length
		if typ == "IEND" {
			break
		}
	}

	return chunks, nil
}

func ListChunks(b []byte) ([]ChunkInfo, error) {
	if len(b) < 8 || !bytes.Equal(b[:8], PNGSignature) {
		return nil, ErrInvalidPNGSignature
	}

	var list []ChunkInfo
	i := 8
	for i+12 <= len(b) {
		offset := int64(i)
		length := int(binary.BigEndian.Uint32(b[i : i+4]))
		if i+12+length > len(b) {
			return nil, fmt.Errorf("%w: chunk starting at offset %d overruns file", ErrCorruptChunk, i)
		}

		typ := string(b[i+4 : i+8])
		storedCRC := binary.BigEndian.Uint32(b[i+8+length : i+12+length])
		body := b[i+4 : i+8+length]
		calcCRC := crc32.ChecksumIEEE(body)

		list = append(list, ChunkInfo{
			Type:        typ,
			Length:      length,
			Offset:      offset,
			CRC:         storedCRC,
			ComputedCRC: calcCRC,
			ValidCRC:    storedCRC == calcCRC,
			IsAncillary: unicode.IsLower(rune(typ[0])),
			IsPrivate:   unicode.IsLower(rune(typ[1])),
		})

		i += 12 + length
		if typ == "IEND" {
			break
		}
	}

	return list, nil
}

func WriteChunks(chunks []Chunk) []byte {
	var out bytes.Buffer
	out.Write(PNGSignature)

	for _, c := range chunks {
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(c.Data)))
		out.Write(lenBuf[:])

		body := append([]byte(c.Type), c.Data...)
		out.Write(body)

		var crcBuf [4]byte
		crc := crc32.ChecksumIEEE(body)
		binary.BigEndian.PutUint32(crcBuf[:], crc)
		out.Write(crcBuf[:])
	}

	return out.Bytes()
}

func EmbedChunk(pngBytes []byte, chunkType string, data []byte) ([]byte, error) {
	if err := ValidateChunkType(chunkType); err != nil {
		return nil, err
	}

	chunks, err := ParseChunks(pngBytes)
	if err != nil {
		return nil, err
	}

	iendIdx := -1
	for idx, c := range chunks {
		if c.Type == "IEND" {
			iendIdx = idx
			break
		}
	}
	if iendIdx == -1 {
		return nil, ErrMissingIEND
	}

	newChunk := Chunk{
		Type: chunkType,
		Data: append([]byte(nil), data...),
	}

	newChunks := make([]Chunk, 0, len(chunks)+1)
	newChunks = append(newChunks, chunks[:iendIdx]...)
	newChunks = append(newChunks, newChunk)
	newChunks = append(newChunks, chunks[iendIdx:]...)

	return WriteChunks(newChunks), nil
}

func ExtractChunks(pngBytes []byte, chunkType string) ([][]byte, error) {
	chunks, err := ParseChunks(pngBytes)
	if err != nil {
		return nil, err
	}

	var results [][]byte
	for _, c := range chunks {
		if c.Type == chunkType {
			results = append(results, c.Data)
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrChunkNotFound, chunkType)
	}

	return results, nil
}

func EmbedTextChunk(pngBytes []byte, keyword string, text string) ([]byte, error) {
	if len(keyword) < 1 || len(keyword) > 79 {
		return nil, ErrInvalidKeyword
	}
	for _, r := range keyword {
		if r < 32 || r > 126 {
			return nil, ErrInvalidKeyword
		}
	}

	var data bytes.Buffer
	data.WriteString(keyword)
	data.WriteByte(0x00)
	data.WriteString(text)

	return EmbedChunk(pngBytes, "tEXt", data.Bytes())
}

func ExtractTextChunks(pngBytes []byte) (map[string]string, error) {
	chunks, err := ExtractChunks(pngBytes, "tEXt")
	if err != nil {
		return nil, err
	}

	results := make(map[string]string)
	for _, raw := range chunks {
		nullIdx := bytes.IndexByte(raw, 0x00)
		if nullIdx > 0 && nullIdx < len(raw) {
			results[string(raw[:nullIdx])] = string(raw[nullIdx+1:])
		}
	}

	return results, nil
}
