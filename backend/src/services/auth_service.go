package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type AuthService struct {
	secret []byte
	ttl    time.Duration
}

func NewAuthService(secret string, ttlSeconds int64) *AuthService {
	if secret == "" {
		secret = "change_me_in_production"
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 72 * 3600
	}

	return &AuthService{
		secret: []byte(secret),
		ttl:    time.Duration(ttlSeconds) * time.Second,
	}
}

func (a *AuthService) GenerateToken(openid string) (string, int64, error) {
	openid = strings.TrimSpace(openid)
	if openid == "" {
		return "", 0, errors.New("openid is required")
	}

	expiry := time.Now().UTC().Add(a.ttl).Unix()
	data := fmt.Sprintf("%s|%d", openid, expiry)
	signature := a.sign(data)
	raw := fmt.Sprintf("%s|%s", data, signature)
	token := base64.RawURLEncoding.EncodeToString([]byte(raw))
	return token, expiry, nil
}

func (a *AuthService) ValidateToken(token string) (string, error) {
	if token == "" {
		return "", errors.New("missing token")
	}

	rawBytes, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", errors.New("invalid token encoding")
	}

	parts := strings.Split(string(rawBytes), "|")
	if len(parts) != 3 {
		return "", errors.New("invalid token format")
	}

	openid := parts[0]
	expiryRaw := parts[1]
	signature := parts[2]
	if openid == "" {
		return "", errors.New("invalid token openid")
	}

	expiry, err := strconv.ParseInt(expiryRaw, 10, 64)
	if err != nil {
		return "", errors.New("invalid token expiry")
	}

	payload := fmt.Sprintf("%s|%d", openid, expiry)
	expected := a.sign(payload)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return "", errors.New("invalid token signature")
	}

	if time.Now().UTC().Unix() > expiry {
		return "", errors.New("token expired")
	}

	return openid, nil
}

func (a *AuthService) sign(data string) string {
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
