package skorozvon

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/lunyashon/sdmp/internal/domain"
	"github.com/lunyashon/sdmp/internal/lib/curl"
)

func (c *Client) DownloadCallRecording(ctx context.Context, callID string) ([]byte, string, error) {
	callID = strings.TrimSpace(callID)
	if !isCallID(callID) {
		return nil, "", fmt.Errorf("%w: call id", domain.ErrInvalidInput)
	}
	if err := c.pace.Wait(ctx); err != nil {
		return nil, "", err
	}
	raw, ctype, code, err := c.getRecording(ctx, callID)
	if err != nil {
		return nil, "", err
	}
	if code == http.StatusUnauthorized {
		_ = c.cache.Del(ctx, tokenKey)
		raw, ctype, code, err = c.getRecording(ctx, callID)
		if err != nil {
			return nil, "", err
		}
	}
	if code != http.StatusOK {
		return nil, "", c.statusError(code, raw)
	}
	if ctype == "" {
		ctype = "audio/mpeg"
	}
	return raw, ctype, nil
}

func (c *Client) getRecording(ctx context.Context, callID string) ([]byte, string, int, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	u := c.cfg.BaseURL + "/api/v2/calls/" + callID + ".mp3?access_token=" + url.QueryEscape(token)
	raw, code, err := curl.CurlWithContext(ctx, c.http, u, http.MethodGet, nil, map[string]string{
		"Accept": "audio/mpeg",
	})
	if err != nil {
		return nil, "", 0, err
	}
	return raw, "audio/mpeg", code, nil
}

func isCallID(s string) bool {
	if s == "" || len(s) > 20 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
