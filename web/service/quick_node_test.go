package service

import "testing"

func TestGenerateX25519KeyPair(t *testing.T) {
	pair, err := GenerateX25519KeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if pair.PrivateKey == "" || pair.PublicKey == "" {
		t.Fatalf("expected non-empty key pair: %#v", pair)
	}
}
