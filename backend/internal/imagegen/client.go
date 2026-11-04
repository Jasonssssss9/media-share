package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultModel = "gpt-image-2.5-flare"
const Timeout = 150 * time.Second
const MaxPromptLength = 1000
const maxResponseSize = 8 << 20

var (
	ErrUnavailable = errors.New("Image generation is not configured.")
	ErrResponse    = errors.New("The image service returned an invalid response.")
)

type APIError struct {
	Status    int
	Code      string
	Type      string
	RequestID string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("image API returned HTTP %d (code=%s, request_id=%s)", e.Status, e.Code, e.RequestID)
}

type Client struct {
	apiKey   string
	model    string
	endpoint string
	http     *http.Client
}

func NewClient(apiKey, model string) *Client {
	if strings.TrimSpace(model) == "" {
		model = DefaultModel
	}
	return &Client{
		apiKey: strings.TrimSpace(apiKey), model: model,
		endpoint: "https://api.openai.com/v1/images/generations",
		http:     &http.Client{Timeout: Timeout},
	}
}

func (c *Client) Enabled() bool {
	return c.apiKey != ""
}

func (c *Client) Generate(ctx context.Context, prompt string) ([]byte, error) {
	if !c.Enabled() {
		return nil, ErrUnavailable
	}
	body, err := json.Marshal(map[string]any{
		"model": c.model, "prompt": prompt, "n": 1,
		"size": "1024x1024", "quality": "low",
		"output_format": "jpeg", "output_compression": 80,
	})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseSize {
		return nil, ErrResponse
	}
	if response.StatusCode != http.StatusOK {
		var result struct {
			Error struct {
				Code string `json:"code"`
				Type string `json:"type"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &result)
		return nil, &APIError{
			Status: response.StatusCode, Code: result.Error.Code,
			Type: result.Error.Type, RequestID: response.Header.Get("X-Request-ID"),
		}
	}
	var result struct {
		Data []struct {
			Image string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil || len(result.Data) != 1 || result.Data[0].Image == "" {
		return nil, ErrResponse
	}
	image, err := base64.StdEncoding.DecodeString(result.Data[0].Image)
	if err != nil || len(image) == 0 {
		return nil, ErrResponse
	}
	return image, nil
}
