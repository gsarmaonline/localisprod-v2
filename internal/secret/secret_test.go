package secret_test

import (
	"strings"
	"testing"

	"github.com/gsarma/localisprod-v2/internal/secret"
)

func make32ByteKey() []byte {
	return []byte("12345678901234567890123456789012")
}

func TestNew_ValidKey(t *testing.T) {
	c, err := secret.New(make32ByteKey())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cipher")
	}
}

func TestNew_ShortKey(t *testing.T) {
	_, err := secret.New([]byte("tooshort"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestNew_LongKey(t *testing.T) {
	_, err := secret.New([]byte("this-key-is-way-too-long-for-aes-256-gcm-encryption"))
	if err == nil {
		t.Fatal("expected error for long key")
	}
}

func TestNew_EmptyKey(t *testing.T) {
	_, err := secret.New([]byte{})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	c, err := secret.New(make32ByteKey())
	if err != nil {
		t.Fatal(err)
	}

	plaintext := `{"FOO":"bar","SECRET":"mysecretvalue"}`
	encrypted, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if encrypted == plaintext {
		t.Fatal("encrypted value should differ from plaintext")
	}
	if !strings.HasPrefix(encrypted, "enc:v1:") {
		t.Fatalf("encrypted value should have enc:v1: prefix, got %q", encrypted)
	}

	decrypted, err := c.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("round-trip mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncrypt_ProducesUniqueValues(t *testing.T) {
	c, _ := secret.New(make32ByteKey())
	plaintext := "same input"

	enc1, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	enc2, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	// Due to random nonce, two encryptions of the same value must differ.
	if enc1 == enc2 {
		t.Fatal("expected different ciphertexts for the same plaintext (randomised nonce)")
	}
}

func TestDecrypt_PlaintextPassthrough(t *testing.T) {
	c, _ := secret.New(make32ByteKey())

	// Legacy plaintext (no enc:v1: prefix) should be returned as-is.
	plain := `{"KEY":"VALUE"}`
	result, err := c.Decrypt(plain)
	if err != nil {
		t.Fatalf("decrypt plaintext: %v", err)
	}
	if result != plain {
		t.Fatalf("expected passthrough, got %q", result)
	}
}

func TestDecrypt_EmptyString(t *testing.T) {
	c, _ := secret.New(make32ByteKey())
	result, err := c.Decrypt("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Fatalf("expected empty string, got %q", result)
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	c, _ := secret.New(make32ByteKey())
	_, err := c.Decrypt("enc:v1:!!!notvalidbase64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecrypt_TruncatedCiphertext(t *testing.T) {
	c, _ := secret.New(make32ByteKey())
	// enc:v1: + very short base64 (less than nonce size)
	_, err := c.Decrypt("enc:v1:YWJj") // "abc" in base64 — too short
	if err == nil {
		t.Fatal("expected error for truncated ciphertext")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := []byte("12345678901234567890123456789012")
	key2 := []byte("99999999991234567890123456789012")

	c1, _ := secret.New(key1)
	c2, _ := secret.New(key2)

	encrypted, err := c1.Encrypt("secret data")
	if err != nil {
		t.Fatal(err)
	}

	_, err = c2.Decrypt(encrypted)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestEncryptDecrypt_EmptyString(t *testing.T) {
	c, _ := secret.New(make32ByteKey())
	enc, err := c.Encrypt("")
	if err != nil {
		t.Fatal(err)
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != "" {
		t.Fatalf("expected empty string after round-trip, got %q", dec)
	}
}

func TestEncryptDecrypt_SpecialCharacters(t *testing.T) {
	c, _ := secret.New(make32ByteKey())
	plaintext := "line1\nline2\ttab\r\n unicode: 日本語"
	enc, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != plaintext {
		t.Fatalf("special chars round-trip failed: got %q", dec)
	}
}
