package clientcfg

import (
	"encoding/base64"
	"strings"
)

const uriScheme = "slipnet://"

// Encode takes v16 fields and produces a slipnet:// URI.
func Encode(fields [TotalFields]string) string {
	payload := strings.Join(fields[:], "|")
	encoded := base64.URLEncoding.EncodeToString([]byte(payload))
	return uriScheme + encoded
}

// Decode parses a slipnet:// URI back into v16 fields.
func Decode(uri string) ([TotalFields]string, error) {
	var fields [TotalFields]string

	encoded := strings.TrimPrefix(uri, uriScheme)
	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return fields, err
	}

	parts := strings.Split(string(data), "|")
	for i := 0; i < len(parts) && i < TotalFields; i++ {
		fields[i] = parts[i]
	}

	return fields, nil
}
