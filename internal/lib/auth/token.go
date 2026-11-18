package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const tokenTTL = 14 * 24 * time.Hour

func CheckPassword(login, password, wantLogin, wantPassword string) bool {
	if strings.TrimSpace(wantLogin) == "" || wantPassword == "" {
		return false
	}
	okLogin := subtle.ConstantTimeCompare([]byte(login), []byte(wantLogin)) == 1
	okPass := subtle.ConstantTimeCompare([]byte(password), []byte(wantPassword)) == 1
	return okLogin && okPass
}

func Issue(secret string, now time.Time) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("auth secret is empty")
	}
	exp := now.Add(tokenTTL).Unix()
	payload := strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig, nil
}

func Valid(secret, token string, now time.Time) bool {
	if strings.TrimSpace(secret) == "" || strings.TrimSpace(token) == "" {
		return false
	}
	payload, sig, ok := strings.Cut(token, ".")
	if !ok || payload == "" || sig == "" {
		return false
	}
	exp, err := strconv.ParseInt(payload, 10, 64)
	if err != nil || now.Unix() > exp {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(sig), []byte(want)) == 1
}
