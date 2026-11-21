package google

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	sheetsScope     = "https://www.googleapis.com/auth/spreadsheets"
	defaultTokenURL = "https://oauth2.googleapis.com/token"
	tokenReserve    = 60 * time.Second
)

type Client struct {
	http *http.Client
	now  func() time.Time

	mu     sync.Mutex
	tokens map[string]cachedToken
}

type cachedToken struct {
	access string
	expiry time.Time
}

type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

func New() *Client {
	return &Client{
		http:   &http.Client{Timeout: 30 * time.Second},
		now:    time.Now,
		tokens: map[string]cachedToken{},
	}
}

func (c *Client) AppendRow(ctx context.Context, credentials json.RawMessage, spreadsheetID, sheetName string, values []string) (int, []byte, error) {
	sa, err := parseServiceAccount(credentials)
	if err != nil {
		return 0, nil, err
	}
	spreadsheetID = strings.TrimSpace(spreadsheetID)
	sheetName = strings.TrimSpace(sheetName)
	if spreadsheetID == "" || sheetName == "" {
		return 0, nil, fmt.Errorf("spreadsheet_id and sheet_name are required")
	}

	token, err := c.accessToken(ctx, sa)
	if err != nil {
		return 0, nil, err
	}
	status, body, err := c.postAppend(ctx, token, spreadsheetID, sheetName, values)
	if status == http.StatusUnauthorized {
		c.forget(sa.ClientEmail)
		token, err = c.accessToken(ctx, sa)
		if err != nil {
			return 0, nil, err
		}
		status, body, err = c.postAppend(ctx, token, spreadsheetID, sheetName, values)
	}
	if err != nil {
		return status, body, err
	}
	if status < 200 || status >= 300 {
		return status, body, fmt.Errorf("google sheets HTTP %d: %s", status, googleError(body))
	}
	return status, body, nil
}

// HeaderRow читает названия листов и первую строку. Пустой sheetName — первый лист книги.
func (c *Client) HeaderRow(ctx context.Context, credentials json.RawMessage, spreadsheetID, sheetName string) (string, []string, []string, error) {
	sa, err := parseServiceAccount(credentials)
	if err != nil {
		return "", nil, nil, err
	}
	spreadsheetID = strings.TrimSpace(spreadsheetID)
	if spreadsheetID == "" {
		return "", nil, nil, fmt.Errorf("spreadsheet_id is required")
	}
	token, err := c.accessToken(ctx, sa)
	if err != nil {
		return "", nil, nil, err
	}
	resolved, columns, sheets, status, err := c.readHeader(ctx, token, spreadsheetID, sheetName)
	if status == http.StatusUnauthorized {
		c.forget(sa.ClientEmail)
		token, err = c.accessToken(ctx, sa)
		if err != nil {
			return "", nil, nil, err
		}
		resolved, columns, sheets, status, err = c.readHeader(ctx, token, spreadsheetID, sheetName)
	}
	return resolved, columns, sheets, err
}

func (c *Client) readHeader(ctx context.Context, token, spreadsheetID, sheetName string) (string, []string, []string, int, error) {
	metaStatus, metaBody, err := c.getJSON(ctx, token, "https://sheets.googleapis.com/v4/spreadsheets/"+url.PathEscape(spreadsheetID)+"?fields=sheets.properties.title")
	if err != nil {
		return "", nil, nil, metaStatus, err
	}
	if metaStatus < 200 || metaStatus >= 300 {
		return "", nil, nil, metaStatus, fmt.Errorf("google sheets HTTP %d: %s", metaStatus, googleError(metaBody))
	}
	var meta struct {
		Sheets []struct {
			Properties struct {
				Title string `json:"title"`
			} `json:"properties"`
		} `json:"sheets"`
	}
	if err := json.Unmarshal(metaBody, &meta); err != nil {
		return "", nil, nil, metaStatus, fmt.Errorf("decode spreadsheet: %w", err)
	}
	sheets := make([]string, 0, len(meta.Sheets))
	for _, sheet := range meta.Sheets {
		title := strings.TrimSpace(sheet.Properties.Title)
		if title != "" {
			sheets = append(sheets, title)
		}
	}
	if len(sheets) == 0 {
		return "", nil, nil, metaStatus, fmt.Errorf("в таблице нет листов")
	}
	sheetName = strings.TrimSpace(sheetName)
	if sheetName == "" {
		sheetName = sheets[0]
	} else if !containsSheet(sheets, sheetName) {
		return "", nil, sheets, metaStatus, fmt.Errorf("листа %q нет. В таблице: %s", sheetName, strings.Join(sheets, ", "))
	}

	rangeExpr := "'" + strings.ReplaceAll(sheetName, "'", "''") + "'!1:1"
	status, body, err := c.getJSON(ctx, token, "https://sheets.googleapis.com/v4/spreadsheets/"+url.PathEscape(spreadsheetID)+"/values/"+url.PathEscape(rangeExpr))
	if err != nil {
		return sheetName, nil, sheets, status, err
	}
	if status < 200 || status >= 300 {
		return sheetName, nil, sheets, status, fmt.Errorf("google sheets HTTP %d: %s", status, googleError(body))
	}
	var decoded struct {
		Values [][]string `json:"values"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return sheetName, nil, sheets, status, fmt.Errorf("decode header row: %w", err)
	}
	var row []string
	if len(decoded.Values) > 0 {
		row = decoded.Values[0]
	}
	columns := cleanSheetHeaders(row)
	if len(columns) == 0 {
		return sheetName, nil, sheets, status, fmt.Errorf("в первой строке листа %q нет заголовков", sheetName)
	}
	return sheetName, columns, sheets, status, nil
}

func (c *Client) getJSON(ctx context.Context, token, endpoint string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func containsSheet(sheets []string, name string) bool {
	for _, sheet := range sheets {
		if sheet == name {
			return true
		}
	}
	return false
}

func cleanSheetHeaders(row []string) []string {
	end := len(row)
	for end > 0 && strings.TrimSpace(row[end-1]) == "" {
		end--
	}
	if end == 0 {
		return nil
	}
	if end > 60 {
		end = 60
	}
	out := make([]string, 0, end)
	for i := 0; i < end; i++ {
		header := strings.TrimSpace(row[i])
		if header == "" {
			header = fmt.Sprintf("Колонка %d", i+1)
		}
		out = append(out, header)
	}
	return out
}

func (c *Client) postAppend(ctx context.Context, token, spreadsheetID, sheetName string, values []string) (int, []byte, error) {
	rangeExpr := "'" + strings.ReplaceAll(sheetName, "'", "''") + "'!A:ZZ"
	endpoint := "https://sheets.googleapis.com/v4/spreadsheets/" +
		url.PathEscape(spreadsheetID) +
		"/values/" + url.PathEscape(rangeExpr) +
		":append?valueInputOption=RAW&insertDataOption=INSERT_ROWS"

	payload, err := json.Marshal(map[string]any{"values": [][]string{values}})
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func (c *Client) accessToken(ctx context.Context, sa serviceAccount) (string, error) {
	now := c.now()
	c.mu.Lock()
	cached, ok := c.tokens[sa.ClientEmail]
	c.mu.Unlock()
	if ok && cached.access != "" && now.Add(tokenReserve).Before(cached.expiry) {
		return cached.access, nil
	}

	assertion, err := signJWT(sa, now)
	if err != nil {
		return "", err
	}
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sa.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("google token HTTP %d: %s", resp.StatusCode, googleError(body))
	}
	var decoded struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("decode google token: %w", err)
	}
	if decoded.AccessToken == "" {
		return "", fmt.Errorf("google did not return access_token")
	}
	ttl := time.Duration(decoded.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	c.mu.Lock()
	c.tokens[sa.ClientEmail] = cachedToken{access: decoded.AccessToken, expiry: now.Add(ttl)}
	c.mu.Unlock()
	return decoded.AccessToken, nil
}

func (c *Client) forget(email string) {
	c.mu.Lock()
	delete(c.tokens, email)
	c.mu.Unlock()
}

func parseServiceAccount(raw json.RawMessage) (serviceAccount, error) {
	var sa serviceAccount
	if err := json.Unmarshal(raw, &sa); err != nil {
		return serviceAccount{}, fmt.Errorf("decode service account: %w", err)
	}
	sa.ClientEmail = strings.TrimSpace(sa.ClientEmail)
	sa.PrivateKey = strings.TrimSpace(sa.PrivateKey)
	sa.TokenURI = strings.TrimSpace(sa.TokenURI)
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return serviceAccount{}, fmt.Errorf("service account client_email and private_key are required")
	}
	if sa.TokenURI == "" {
		sa.TokenURI = defaultTokenURL
	}
	return sa, nil
}

func signJWT(sa serviceAccount, now time.Time) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(map[string]any{
		"iss":   sa.ClientEmail,
		"scope": sheetsScope,
		"aud":   sa.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	if err != nil {
		return "", err
	}
	unsigned := b64url(header) + "." + b64url(claims)
	key, err := parseRSAPrivateKey(sa.PrivateKey)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return unsigned + "." + b64url(sig), nil
}

func parseRSAPrivateKey(pemKey string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, fmt.Errorf("invalid private_key pem")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private_key is not rsa")
		}
		return rsaKey, nil
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private_key: %w", err)
	}
	return key, nil
}

func googleError(body []byte) string {
	var decoded struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err == nil && decoded.Error.Message != "" {
		return decoded.Error.Message
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return "empty body"
	}
	return msg
}

func b64url(raw []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(raw), "=")
}
