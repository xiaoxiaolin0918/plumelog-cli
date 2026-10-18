//go:build !windows

package credstore

import "fmt"

const schemeAESLocal = "aes-local-v1"

// protect encrypts with AES-256-GCM using a local key file (mode 0600).
// This is OS file-permission protection, not a hardware TPM/keyring bound secret.
func protect(plain []byte, configDir string) ([]byte, string, error) {
	key, err := loadOrCreateLocalKey(configDir)
	if err != nil {
		return nil, "", err
	}
	blob, err := sealAESGCM(key, plain)
	if err != nil {
		return nil, "", err
	}
	return blob, schemeAESLocal, nil
}

func unprotect(blob []byte, scheme, configDir string) ([]byte, error) {
	if scheme != "" && scheme != schemeAESLocal {
		return nil, fmt.Errorf("credstore: unsupported protect scheme %q", scheme)
	}
	key, err := loadOrCreateLocalKey(configDir)
	if err != nil {
		return nil, err
	}
	return openAESGCM(key, blob)
}
