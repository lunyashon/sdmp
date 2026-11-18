package recordinglink

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// Sign — постоянная подпись uuid. Ссылка в Битрикс не протухает, пока секрет тот же.
func Sign(secret, leadUUID string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.TrimSpace(leadUUID)))
	return hex.EncodeToString(mac.Sum(nil))
}

func Valid(secret, leadUUID, sig string) bool {
	secret = strings.TrimSpace(secret)
	sig = strings.TrimSpace(sig)
	if secret == "" || sig == "" || strings.TrimSpace(leadUUID) == "" {
		return false
	}
	want := Sign(secret, leadUUID)
	if len(want) != len(sig) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(want), []byte(sig)) == 1
}

// URL — публичный прокси записи. Пусто, если нет base/secret/uuid.
func URL(base, secret, leadUUID string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	leadUUID = strings.TrimSpace(leadUUID)
	if base == "" || strings.TrimSpace(secret) == "" || leadUUID == "" {
		return ""
	}
	return base + "/rest/recordings/" + leadUUID + ".mp3?sig=" + Sign(secret, leadUUID)
}
