package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"

	"gorm.io/gorm"
)

const (
	legacyPasswordHashPrefix = "sha256"
	passwordHashPrefix       = "pbkdf2_sha256"
	passwordHashIter         = 210000
	passwordSaltSize         = 16
	passwordKeySize          = 32
	minPasswordLength        = 8
	maxPasswordLength        = 128
)

type UserService struct {
}

func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) CheckUser(username string, password string) *model.User {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		Where("username = ?", username).
		First(user).
		Error
	if err == gorm.ErrRecordNotFound {
		return nil
	} else if err != nil {
		logger.Warning("check user err:", err)
		return nil
	}
	if !verifyPassword(user.Password, password) {
		return nil
	}
	if !isCurrentPasswordHash(user.Password) {
		if hashed, err := hashPassword(password); err == nil {
			_ = db.Model(model.User{}).Where("id = ?", user.Id).Update("password", hashed).Error
			user.Password = hashed
		} else {
			logger.Warning("upgrade password hash failed:", err)
		}
	}
	return user
}

func (s *UserService) UpdateUser(id int, username string, password string) error {
	if err := validateUserCredentials(username, password); err != nil {
		return err
	}
	hashed, err := hashPassword(password)
	if err != nil {
		return err
	}
	db := database.GetDB()
	return db.Model(model.User{}).
		Where("id = ?", id).
		Update("username", username).
		Update("password", hashed).
		Error
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if err := validateUserCredentials(username, password); err != nil {
		return err
	}
	hashed, err := hashPassword(password)
	if err != nil {
		return err
	}
	db := database.GetDB()
	user := &model.User{}
	err = db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = hashed
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashed
	return db.Save(user).Error
}

func isPasswordHash(password string) bool {
	return strings.HasPrefix(password, passwordHashPrefix+"$") || strings.HasPrefix(password, legacyPasswordHashPrefix+"$")

}

func isCurrentPasswordHash(password string) bool {
	return strings.HasPrefix(password, passwordHashPrefix+"$")
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := pbkdf2SHA256([]byte(password), salt, passwordHashIter, passwordKeySize)
	return fmt.Sprintf("%s$%d$%s$%s", passwordHashPrefix, passwordHashIter,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(stored string, password string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 {
		return subtle.ConstantTimeCompare([]byte(stored), []byte(password)) == 1
	}
	if parts[0] == legacyPasswordHashPrefix {
		return verifyLegacyPasswordHash(parts, password)
	}
	if parts[0] != passwordHashPrefix {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual := pbkdf2SHA256([]byte(password), salt, iter, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func verifyLegacyPasswordHash(parts []string, password string) bool {
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	actual := stretchPassword([]byte(password), salt, iter)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func validateUserCredentials(username string, password string) error {
	if strings.TrimSpace(username) == "" {
		return errors.New("username can not be empty")
	}
	if password == "" {
		return errors.New("password can not be empty")
	}
	if len(password) < minPasswordLength || len(password) > maxPasswordLength {
		return fmt.Errorf("password length must be between %d and %d characters", minPasswordLength, maxPasswordLength)
	}

	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	classes := 0
	for _, ok := range []bool{hasLower, hasUpper, hasDigit, hasSymbol} {
		if ok {
			classes++
		}
	}
	if classes < 3 {
		return errors.New("password must include at least three of lowercase letters, uppercase letters, digits, and symbols")
	}
	return nil
}

func pbkdf2SHA256(password []byte, salt []byte, iter int, keyLen int) []byte {
	hLen := sha256.Size
	numBlocks := int(math.Ceil(float64(keyLen) / float64(hLen)))
	dk := make([]byte, 0, numBlocks*hLen)
	for block := 1; block <= numBlocks; block++ {
		u := pbkdf2F(password, salt, iter, block)
		dk = append(dk, u...)
	}
	return dk[:keyLen]
}

func pbkdf2F(password []byte, salt []byte, iter int, blockNum int) []byte {
	mac := hmac.New(sha256.New, password)
	mac.Write(salt)
	mac.Write([]byte{byte(blockNum >> 24), byte(blockNum >> 16), byte(blockNum >> 8), byte(blockNum)})
	u := mac.Sum(nil)
	out := append([]byte{}, u...)
	for i := 1; i < iter; i++ {
		mac = hmac.New(sha256.New, password)
		mac.Write(u)
		u = mac.Sum(nil)
		for j := range out {
			out[j] ^= u[j]
		}
	}
	return out
}

func stretchPassword(password []byte, salt []byte, iter int) []byte {
	h := sha256.Sum256(append(append([]byte{}, salt...), password...))
	buf := h[:]
	for i := 1; i < iter; i++ {
		next := sha256.Sum256(buf)
		buf = next[:]
	}
	return buf
}
