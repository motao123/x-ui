package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	keySize    = 32 // AES-256
	nonceSize  = 12 // GCM standard nonce
	aesKeyFile = "encryption.key"
)

// LoadOrCreateKey 从 keyPath 加载 AES-256 密钥，不存在则生成
func LoadOrCreateKey(keyPath string) ([]byte, error) {
	if data, err := os.ReadFile(keyPath); err == nil {
		if len(data) != keySize {
			return nil, fmt.Errorf("encryption key file has invalid size %d, expected %d", len(data), keySize)
		}
		return data, nil
	}

	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate encryption key failed: %w", err)
	}

	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create key directory failed: %w", err)
	}

	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return nil, fmt.Errorf("write encryption key failed: %w", err)
	}
	return key, nil
}

// KeyPathFromDBPath 从数据库路径推导密钥文件路径
// /etc/x-ui/x-ui.db → /etc/x-ui/encryption.key
func KeyPathFromDBPath(dbPath string) string {
	dir := filepath.Dir(dbPath)
	return filepath.Join(dir, aesKeyFile)
}

// Encrypt AES-256-GCM 加密，输出 hex(非ce + 密文)
func Encrypt(plaintext []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	return hex.EncodeToString(append(nonce, ciphertext...)), nil
}

// Decrypt AES-256-GCM 解密，输入 hex(非ce + 密文)
func Decrypt(cipherHex string, key []byte) ([]byte, error) {
	data, err := hex.DecodeString(cipherHex)
	if err != nil {
		return nil, errors.New("ciphertext is not valid hex")
	}
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("decryption failed (key mismatch or data corrupted)")
	}
	return plaintext, nil
}

// IsEncrypted 检测字符串是否为加密格式（hex 编码且长度足够）
func IsEncrypted(s string) bool {
	if len(s) < nonceSize*2 { // hex 编码后至少需要 24 字符
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

