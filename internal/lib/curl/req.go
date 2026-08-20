package curl

import (
	"context"
	"io"
	"net/http"
)

func Curl(
	client *http.Client,
	url string,
	method string,
	bodyReader io.Reader,
	headers map[string]string,
) ([]byte, int, error) {

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer clearResponse(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	return body, resp.StatusCode, nil
}

func CurlWithContext(ctx context.Context, client *http.Client, url string, method string, bodyReader io.Reader, headers map[string]string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer clearResponse(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}

func clearResponse(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
