package internal

import (
	"encoding/base64"
	"errors"
	"strings"
)

const maxProfileImageBytes = 650 * 1024

func decodeProfileImageBlob(value string) ([]byte, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, "", nil
	}
	prefixes := map[string]string{
		"data:image/jpeg;base64,": "image/jpeg",
		"data:image/png;base64,":  "image/png",
		"data:image/webp;base64,": "image/webp",
	}
	for prefix, mime := range prefixes {
		if !strings.HasPrefix(value, prefix) {
			continue
		}
		blob, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
		if err != nil {
			return nil, "", errors.New("profile image data is invalid")
		}
		if len(blob) == 0 || len(blob) > maxProfileImageBytes {
			return nil, "", errors.New("profile image is too large")
		}
		return blob, mime, nil
	}
	return nil, "", errors.New("profile image must be a JPEG, PNG, or WebP image")
}

func userProfileImageURL(legacy string, blob []byte, mime string) string {
	mime = strings.TrimSpace(strings.ToLower(mime))
	if len(blob) > 0 && (mime == "image/jpeg" || mime == "image/png" || mime == "image/webp") {
		return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(blob)
	}
	return publicImageURL(legacy)
}
