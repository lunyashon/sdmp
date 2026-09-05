package id

import (
	"crypto/rand"
	"fmt"
)

const webhookTokenAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func NewWebhookToken() (string, error) {
	const n = 16
	var raw [n]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = webhookTokenAlphabet[int(raw[i])%len(webhookTokenAlphabet)]
	}
	return string(out), nil
}

func NewUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
