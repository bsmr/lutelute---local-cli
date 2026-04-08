package tool

import (
	"bytes"
	"io"
	"os"
)

const binaryDetectSize = 8192

// IsBinaryFile checks if the open file appears to be binary by scanning
// for null bytes in the first 8192 bytes. Seeks back to start after check.
func IsBinaryFile(f *os.File) (bool, error) {
	header := make([]byte, binaryDetectSize)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return false, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	return bytes.ContainsRune(header[:n], 0), nil
}

// ToInt converts a JSON-deserialized number (float64, int, int64) to int.
func ToInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}
