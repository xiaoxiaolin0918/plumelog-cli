//go:build windows

package credstore

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const schemeDPAPI = "dpapi"

// protect encrypts plaintext with Windows DPAPI (current user scope).
func protect(plain []byte, _ string) ([]byte, string, error) {
	if len(plain) == 0 {
		return nil, "", errors.New("credstore: empty password")
	}
	in := windows.DataBlob{
		Size: uint32(len(plain)),
		Data: &plain[0],
	}
	var out windows.DataBlob
	err := windows.CryptProtectData(&in, nil, nil, 0, nil, 0, &out)
	if err != nil {
		return nil, "", fmt.Errorf("credstore: DPAPI protect: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	cp := make([]byte, out.Size)
	copy(cp, unsafe.Slice(out.Data, out.Size))
	return cp, schemeDPAPI, nil
}

func unprotect(blob []byte, scheme, _ string) ([]byte, error) {
	if scheme != "" && scheme != schemeDPAPI {
		return nil, fmt.Errorf("credstore: unsupported protect scheme %q on windows", scheme)
	}
	if len(blob) == 0 {
		return nil, errors.New("credstore: empty ciphertext")
	}
	in := windows.DataBlob{
		Size: uint32(len(blob)),
		Data: &blob[0],
	}
	var out windows.DataBlob
	err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, 0, &out)
	if err != nil {
		return nil, fmt.Errorf("credstore: DPAPI unprotect: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	cp := make([]byte, out.Size)
	copy(cp, unsafe.Slice(out.Data, out.Size))
	return cp, nil
}
