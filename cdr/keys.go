package cdr

import "fmt"

// SerializedKeyExtractor wraps a KeyFieldExtractor into a function that
// extracts and concatenates key field bytes from a CDR sample.
// The returned function is suitable for use as ddswsclient.KeyExtractFunc.
func SerializedKeyExtractor(ext KeyFieldExtractor) func([]byte) ([]byte, error) {
	return func(data []byte) ([]byte, error) {
		fields, err := ext.ExtractKeyFields(data)
		if err != nil {
			return nil, err
		}
		if len(fields) == 0 {
			return nil, nil
		}
		var buf []byte
		for _, f := range fields {
			start := int(f.Offset)
			end := start + int(f.Size)
			if end > len(data) {
				return nil, fmt.Errorf("key field at offset %d size %d exceeds data length %d", f.Offset, f.Size, len(data))
			}
			buf = append(buf, data[start:end]...)
		}
		return buf, nil
	}
}
