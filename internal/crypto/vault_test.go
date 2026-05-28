package crypto

import (
	"strings"
	"testing"
)

func TestNewEmptyPassword(t *testing.T) {
	_, err := New("")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestValidCreation(t *testing.T) {
	v, err := New("test-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil vault")
	}
	if v.aead == nil {
		t.Error("expected AEAD to be initialized")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	v, err := New("secret-password")
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "hello world"
	encrypted, err := v.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if encrypted == "" {
		t.Fatal("expected non-empty ciphertext")
	}
	if encrypted == plaintext {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := v.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	v, err := New("password")
	if err != nil {
		t.Fatal(err)
	}

	_, err = v.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecryptInvalidCipher(t *testing.T) {
	v, err := New("password")
	if err != nil {
		t.Fatal(err)
	}

	// Valid base64 but too short to contain nonce + ciphertext
	_, err = v.Decrypt("YQ==")
	if err != ErrInvalidCipher {
		t.Errorf("expected ErrInvalidCipher, got %v", err)
	}
}

func TestSetGet(t *testing.T) {
	v, err := New("master-key")
	if err != nil {
		t.Fatal(err)
	}

	err = v.Set("api-key", "sk-12345")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := v.Get("api-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "sk-12345" {
		t.Errorf("expected 'sk-12345', got %q", val)
	}
}

func TestGetNonExistent(t *testing.T) {
	v, err := New("key")
	if err != nil {
		t.Fatal(err)
	}

	_, err = v.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent key")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' message, got: %s", err.Error())
	}
}

func TestDelete(t *testing.T) {
	v, err := New("key")
	if err != nil {
		t.Fatal(err)
	}

	v.Set("temp", "value")
	if !v.Exists("temp") {
		t.Fatal("expected key to exist")
	}

	v.Delete("temp")
	if v.Exists("temp") {
		t.Error("expected key to be deleted")
	}
}

func TestKeys(t *testing.T) {
	v, err := New("key")
	if err != nil {
		t.Fatal(err)
	}

	v.Set("a", "1")
	v.Set("b", "2")
	v.Set("c", "3")

	keys := v.Keys()
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d: %v", len(keys), keys)
	}

	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for _, expected := range []string{"a", "b", "c"} {
		if !keySet[expected] {
			t.Errorf("expected key %q not found", expected)
		}
	}
}

func TestKeysEmpty(t *testing.T) {
	v, err := New("key")
	if err != nil {
		t.Fatal(err)
	}

	keys := v.Keys()
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestExists(t *testing.T) {
	v, err := New("key")
	if err != nil {
		t.Fatal(err)
	}

	if v.Exists("missing") {
		t.Error("expected false for missing key")
	}

	v.Set("present", "value")
	if !v.Exists("present") {
		t.Error("expected true for existing key")
	}
}

func TestRotateSecret(t *testing.T) {
	v, err := New("key")
	if err != nil {
		t.Fatal(err)
	}

	v.Set("token", "old-value")
	val, _ := v.Get("token")
	if val != "old-value" {
		t.Fatal("initial value not set correctly")
	}

	err = v.RotateSecret("token", "new-value")
	if err != nil {
		t.Fatalf("RotateSecret failed: %v", err)
	}

	val, err = v.Get("token")
	if err != nil {
		t.Fatalf("Get after rotate failed: %v", err)
	}
	if val != "new-value" {
		t.Errorf("expected 'new-value', got %q", val)
	}
}

func TestClear(t *testing.T) {
	v, err := New("key")
	if err != nil {
		t.Fatal(err)
	}

	v.Set("a", "1")
	v.Set("b", "2")
	if v.Count() != 2 {
		t.Fatalf("expected 2 secrets, got %d", v.Count())
	}

	v.Clear()
	if v.Count() != 0 {
		t.Errorf("expected 0 secrets after clear, got %d", v.Count())
	}
}

func TestCount(t *testing.T) {
	v, err := New("key")
	if err != nil {
		t.Fatal(err)
	}

	if v.Count() != 0 {
		t.Errorf("expected 0, got %d", v.Count())
	}

	v.Set("x", "y")
	if v.Count() != 1 {
		t.Errorf("expected 1, got %d", v.Count())
	}

	v.Set("z", "w")
	if v.Count() != 2 {
		t.Errorf("expected 2, got %d", v.Count())
	}
}

func TestExportImport(t *testing.T) {
	v1, err := New("shared-key")
	if err != nil {
		t.Fatal(err)
	}

	v1.Set("secret-a", "value-a")
	v1.Set("secret-b", "value-b")

	exported := v1.Export()
	if len(exported) != 2 {
		t.Fatalf("expected 2 exported secrets, got %d", len(exported))
	}

	v2, err := New("shared-key")
	if err != nil {
		t.Fatal(err)
	}

	v2.Import(exported)
	if v2.Count() != 2 {
		t.Fatalf("expected 2 imported secrets, got %d", v2.Count())
	}

	valA, _ := v2.Get("secret-a")
	valB, _ := v2.Get("secret-b")
	if valA != "value-a" || valB != "value-b" {
		t.Errorf("imported values don't match: a=%q, b=%q", valA, valB)
	}
}

func TestExportReturnsCopy(t *testing.T) {
	v, err := New("key")
	if err != nil {
		t.Fatal(err)
	}

	v.Set("s", "val")
	exported := v.Export()

	// Modify exported map
	delete(exported, "s")

	// Original should be unchanged
	if !v.Exists("s") {
		t.Error("export should return a copy, modifications should not affect original")
	}
}

func TestGenerateKey(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	if key1 == "" {
		t.Fatal("expected non-empty key")
	}

	key2, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if key1 == key2 {
		t.Fatal("expected different keys on each call")
	}
}

func TestMultipleEncryptDecrypt(t *testing.T) {
	v, err := New("password")
	if err != nil {
		t.Fatal(err)
	}

	inputs := []string{
		"hello",
		"world",
		"12345",
		"special chars: !@#$%^&*()",
		"unicode: 你好世界",
		"",
	}

	for _, input := range inputs {
		encrypted, err := v.Encrypt(input)
		if err != nil {
			t.Errorf("encrypt(%q) failed: %v", input, err)
			continue
		}

		decrypted, err := v.Decrypt(encrypted)
		if err != nil {
			t.Errorf("decrypt(%q) failed: %v", input, err)
			continue
		}

		if decrypted != input {
			t.Errorf("round-trip failed for %q: got %q", input, decrypted)
		}
	}
}

func TestEncryptProducesDifferentOutputs(t *testing.T) {
	v, err := New("key")
	if err != nil {
		t.Fatal(err)
	}

	enc1, _ := v.Encrypt("same plaintext")
	enc2, _ := v.Encrypt("same plaintext")

	if enc1 == enc2 {
		t.Error("encrypt should produce different outputs due to random nonce")
	}
}

func TestVaultStructure(t *testing.T) {
	v, err := New("test")
	if err != nil {
		t.Fatal(err)
	}

	// Initial state
	if len(v.cache) != 0 {
		t.Error("expected empty cache")
	}
}

func TestSetOverwritesExisting(t *testing.T) {
	v, err := New("key")
	if err != nil {
		t.Fatal(err)
	}

	v.Set("key1", "first")
	v.Set("key1", "second")

	val, _ := v.Get("key1")
	if val != "second" {
		t.Errorf("expected 'second', got %q", val)
	}
	if v.Count() != 1 {
		t.Errorf("expected 1 secret, got %d", v.Count())
	}
}
