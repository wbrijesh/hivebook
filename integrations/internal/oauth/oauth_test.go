package oauth

import (
	"bytes"
	"testing"
	"time"
)

func TestCipherRoundTrip(t *testing.T) {
	c, err := NewCipher(bytes.Repeat([]byte("k"), 32))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := c.EncryptString("ya29.secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(enc, []byte("secret-token")) {
		t.Fatal("ciphertext leaks the plaintext")
	}
	got, err := c.DecryptString(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ya29.secret-token" {
		t.Fatalf("round-trip mismatch: %q", got)
	}
}

func TestCipherEmpty(t *testing.T) {
	c, _ := NewCipher(bytes.Repeat([]byte("k"), 32))
	enc, _ := c.EncryptString("")
	if enc != nil {
		t.Fatalf("empty should encrypt to nil, got %v", enc)
	}
	got, _ := c.DecryptString(nil)
	if got != "" {
		t.Fatalf("nil should decrypt to empty, got %q", got)
	}
}

func TestCipherWrongKeyFails(t *testing.T) {
	a, _ := NewCipher(bytes.Repeat([]byte("a"), 32))
	b, _ := NewCipher(bytes.Repeat([]byte("b"), 32))
	enc, _ := a.EncryptString("secret")
	if _, err := b.DecryptString(enc); err == nil {
		t.Fatal("decrypt with wrong key should fail")
	}
}

func TestNewCipherKeyLength(t *testing.T) {
	if _, err := NewCipher([]byte("too-short")); err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestStateSignVerify(t *testing.T) {
	s, err := NewSigner([]byte("state-key"))
	if err != nil {
		t.Fatal(err)
	}
	tok, err := s.Sign("tenant-1", "us-east", "gdocs", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if st.Tenant != "tenant-1" || st.Region != "us-east" || st.Connector != "gdocs" {
		t.Fatalf("state round-trip mismatch: %+v", st)
	}
}

func TestStateTampered(t *testing.T) {
	s, _ := NewSigner([]byte("state-key"))
	tok, _ := s.Sign("t", "r", "c", time.Minute)
	tampered := tok[:len(tok)-1] + "X"
	if _, err := s.Verify(tampered); err == nil {
		t.Fatal("tampered state should fail verification")
	}
}

func TestStateExpired(t *testing.T) {
	s, _ := NewSigner([]byte("state-key"))
	tok, _ := s.Sign("t", "r", "c", -time.Minute) // already expired
	if _, err := s.Verify(tok); err == nil {
		t.Fatal("expired state should fail verification")
	}
}

func TestStateWrongKey(t *testing.T) {
	s1, _ := NewSigner([]byte("key-one"))
	s2, _ := NewSigner([]byte("key-two"))
	tok, _ := s1.Sign("t", "r", "c", time.Minute)
	if _, err := s2.Verify(tok); err == nil {
		t.Fatal("verify with wrong key should fail")
	}
}
