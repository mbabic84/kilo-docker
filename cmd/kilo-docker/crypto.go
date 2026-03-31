package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// encryptAES encrypts plaintext using AES-256-CBC with PBKDF2 key derivation.
// The output format is compatible with `openssl enc -aes-256-cbc -salt -pbkdf2`:
// it prepends "Salted__" + 8-byte salt, followed by IV + ciphertext with PKCS7 padding.
// This allows interoperability with the original bash-based encryption.
func encryptAES(plaintext []byte, password string) ([]byte, error) {
	salt := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	key := pbkdf2.Key([]byte(password), salt, 10000, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	plaintext = pkcs7Pad(plaintext, aes.BlockSize)

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext[aes.BlockSize:], plaintext)

	result := make([]byte, 0, 16+len(ciphertext))
	result = append(result, []byte("Salted__")...)
	result = append(result, salt...)
	result = append(result, ciphertext...)

	return result, nil
}

// decryptAES decrypts ciphertext produced by encryptAES. It expects the
// "Salted__" header, extracts salt and IV, derives the key via PBKDF2,
// and removes PKCS7 padding. Returns the original plaintext.
// decryptAES decrypts ciphertext produced by encryptAES. It expects the
// "Salted__" header, extracts salt and IV, derives the key via PBKDF2,
// and removes PKCS7 padding. Returns the original plaintext.
func decryptAES(ciphertext []byte, password string) ([]byte, error) {
	if len(ciphertext) < 16 || string(ciphertext[:8]) != "Salted__" {
		return nil, fmt.Errorf("invalid encrypted data format")
	}

	salt := ciphertext[8:16]
	data := ciphertext[16:]

	key := pbkdf2.Key([]byte(password), salt, 10000, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := data[:aes.BlockSize]
	encrypted := data[aes.BlockSize:]

	if len(encrypted)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(encrypted, encrypted)

	plaintext, err := pkcs7Unpad(encrypted)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// pkcs7Pad pads data to the given block size using PKCS#7 padding.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	pad := make([]byte, len(data)+padding)
	copy(pad, data)
	for i := len(data); i < len(pad); i++ {
		pad[i] = byte(padding)
	}
	return pad
}

// pkcs7Unpad removes PKCS#7 padding from data. Returns an error if the
// padding is invalid.
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > len(data) {
		return nil, fmt.Errorf("invalid padding")
	}
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, fmt.Errorf("invalid padding")
		}
	}
	return data[:len(data)-padding], nil
}
