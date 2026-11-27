package llamaserve

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// ContextForModel caps an explicit context at the GGUF's declared window.
// Older or unreadable headers retain the existing 128K ceiling.
func ContextForModel(modelPath string, requested int) int {
	maximum := CtxMax
	if f, err := os.Open(modelPath); err == nil {
		defer f.Close()
		// Only metadata is consulted. Stop early even for an unsupported
		// or malformed file with a very large metadata block.
		if size := ggufContext(io.NewSectionReader(f, 0, 32<<20)); size > 0 {
			maximum = size
		}
	}
	if requested == CtxModelMax {
		return maximum
	}
	return min(requested, maximum)
}

func ggufContext(r io.ReadSeeker) int {
	var header struct {
		Magic   [4]byte
		Version uint32
		Tensors uint64
		Fields  uint64
	}
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil ||
		header.Magic != [4]byte{'G', 'G', 'U', 'F'} || (header.Version != 2 && header.Version != 3) {
		return 0
	}
	contexts := map[string]uint32{}
	var architecture string
	for range header.Fields {
		key, err := readGGUFString(r)
		if err != nil {
			return 0
		}
		var kind uint32
		if err := binary.Read(r, binary.LittleEndian, &kind); err != nil {
			return 0
		}
		switch {
		case key == "general.architecture" && kind == 8:
			architecture, err = readGGUFString(r)
		case strings.HasSuffix(key, ".context_length") && kind == 4:
			var length uint32
			err = binary.Read(r, binary.LittleEndian, &length)
			contexts[key] = length
		default:
			err = skipGGUFValue(r, kind, 1)
		}
		if err != nil {
			return 0
		}
		if size := contexts[architecture+".context_length"]; architecture != "" && size > 0 {
			return int(size)
		}
	}
	return 0
}

func readGGUFString(r io.Reader) (string, error) {
	var size uint64
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return "", err
	}
	if size > 4096 {
		return "", fmt.Errorf("GGUF metadata key or architecture is too long")
	}
	value := make([]byte, size)
	_, err := io.ReadFull(r, value)
	return string(value), err
}

func skipGGUFValue(r io.ReadSeeker, kind uint32, count uint64) error {
	sizes := [...]uint64{1, 1, 2, 2, 4, 4, 4, 1, 0, 0, 8, 8, 8}
	switch kind {
	case 8: // String
		for range count {
			var size uint64
			if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
				return err
			}
			if size > 32<<20 {
				return fmt.Errorf("GGUF metadata string exceeds the header budget")
			}
			if _, err := r.Seek(int64(size), io.SeekCurrent); err != nil {
				return err
			}
		}
		return nil
	case 9: // Array
		var array struct {
			Kind uint32
			Size uint64
		}
		if err := binary.Read(r, binary.LittleEndian, &array); err != nil {
			return err
		}
		if array.Kind == 9 {
			return fmt.Errorf("nested GGUF metadata array")
		}
		return skipGGUFValue(r, array.Kind, array.Size)
	default:
		if kind >= uint32(len(sizes)) || count > (32<<20)/sizes[kind] {
			return fmt.Errorf("invalid GGUF metadata value")
		}
		_, err := r.Seek(int64(count*sizes[kind]), io.SeekCurrent)
		return err
	}
}
