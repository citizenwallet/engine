package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"

	"github.com/ethereum/go-ethereum/crypto"
)

func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// Encrypt takes a secret value and a key, encrypts the value using AES-CFB,
// and returns the encrypted value as a hex-encoded string.
//
// Parameters:
//   - secretValue: The string to be encrypted
//   - key: The encryption key as a hex-encoded string
//
// Returns:
//   - A hex-encoded string of the encrypted value
//   - An error if encryption fails
func Encrypt(secretValue string, key string) (string, error) {
	// Convert the hex-encoded key to an ECDSA private key
	ecdsaKey, err := crypto.HexToECDSA(key)
	if err != nil {
		return "", err
	}

	// Create a new AES cipher using the private key's D value
	block, err := aes.NewCipher(ecdsaKey.D.Bytes())
	if err != nil {
		return "", err
	}

	// Convert the secret value to a byte slice
	plaintext := []byte(secretValue)

	// Create a byte slice to hold the IV and ciphertext
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))

	// Extract the first 16 bytes to use as the IV
	iv := ciphertext[:aes.BlockSize]

	// Fill the IV with random bytes
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	// Create a new CFB encrypter using the AES block cipher and IV
	stream := cipher.NewCFBEncrypter(block, iv)

	// Encrypt the plaintext and store it in the ciphertext slice
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	// Convert the entire ciphertext (including IV) to a hex-encoded string
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt takes an encrypted value and a key, decrypts the value using AES-CFB,
// and returns the decrypted value as a string.
//
// Parameters:
//   - encryptedValue: The hex-encoded string of the encrypted value
//   - key: The decryption key as a hex-encoded string
//
// Returns:
//   - The decrypted string
//   - An error if decryption fails
func Decrypt(encryptedValue string, key string) (string, error) {
	// Convert the hex-encoded key to an ECDSA private key
	ecdsaKey, err := crypto.HexToECDSA(key)
	if err != nil {
		return "", err
	}

	// Decode the hex-encoded encrypted value to bytes
	ciphertext, err := hex.DecodeString(encryptedValue)
	if err != nil {
		return "", err
	}

	// Create a new AES cipher using the private key's D value
	block, err := aes.NewCipher(ecdsaKey.D.Bytes())
	if err != nil {
		return "", err
	}

	// Ensure the ciphertext is long enough to contain the IV
	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	// Extract the IV from the beginning of the ciphertext
	iv := ciphertext[:aes.BlockSize]
	// Remove the IV from the ciphertext
	ciphertext = ciphertext[aes.BlockSize:]

	// Create a new CFB decrypter using the AES block cipher and IV
	stream := cipher.NewCFBDecrypter(block, iv)

	// Decrypt the ciphertext in-place
	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(ciphertext, ciphertext)

	// Convert the decrypted bytes to a string and return
	return string(ciphertext), nil
}
