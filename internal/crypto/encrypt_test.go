package crypto_test

import (
	"encoding/hex"
	"testing"

	"github.com/italomoia/instasae/internal/crypto"
)

const testKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestEncryptDecrypt(t *testing.T) {
	enc, err := crypto.NewEncryptor(testKey)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	original := "my-secret-token"
	ciphertext, err := enc.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	plaintext, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if plaintext != original {
		t.Errorf("got %q, want %q", plaintext, original)
	}
}

func TestEncryptProducesDifferentOutput(t *testing.T) {
	enc, err := crypto.NewEncryptor(testKey)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	input := "same-input"
	c1, err := enc.Encrypt(input)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	c2, err := enc.Encrypt(input)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}

	if c1 == c2 {
		t.Error("two encryptions of the same input should produce different ciphertexts")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	enc1, err := crypto.NewEncryptor(testKey)
	if err != nil {
		t.Fatalf("NewEncryptor key1: %v", err)
	}

	key2 := hex.EncodeToString(make([]byte, 32))
	// key2 is all zeros, different from testKey
	enc2, err := crypto.NewEncryptor(key2)
	if err != nil {
		t.Fatalf("NewEncryptor key2: %v", err)
	}

	ciphertext, err := enc1.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = enc2.Decrypt(ciphertext)
	if err == nil {
		t.Error("decrypting with wrong key should fail")
	}
}

func TestEncryptedDifferentFromPlaintext(t *testing.T) {
	enc, err := crypto.NewEncryptor(testKey)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	plaintext := "visible-token"
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if ciphertext == plaintext {
		t.Error("ciphertext must not equal plaintext")
	}
}

func TestEncryptEmptyString(t *testing.T) {
	enc, err := crypto.NewEncryptor(testKey)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	ciphertext, err := enc.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}

	plaintext, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}

	if plaintext != "" {
		t.Errorf("got %q, want empty string", plaintext)
	}
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	enc, err := crypto.NewEncryptor(testKey)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	_, err = enc.Decrypt("not-valid-base64-ciphertext!!!")
	if err == nil {
		t.Error("decrypting garbage should fail")
	}
}

func TestNewEncryptorInvalidKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"too short", "abcdef"},
		{"not hex", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
		{"empty", ""},
		{"wrong length valid hex", "0123456789abcdef0123456789abcdef"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := crypto.NewEncryptor(tt.key)
			if err == nil {
				t.Errorf("NewEncryptor(%q) should fail", tt.key)
			}
		})
	}
}
