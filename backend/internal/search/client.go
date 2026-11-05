package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const DefaultIndex = "media-share-posts"
const MaxResults = 10000

var indexName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,79}$`)
var ErrResponse = errors.New("Elasticsearch returned an invalid or incomplete response.")

type APIError struct {
	Status int
	Kind   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Elasticsearch returned HTTP %d (%s)", e.Status, e.Kind)
}

type Document struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Hit struct {
	ID        string
	UpdatedAt time.Time
}

type Matches struct {
	Hits  []Hit
	Total int64
}

type Client struct {
	address  string
	password string
	index    string
	http     *http.Client
}

func NewClient(address, password, index string) (*Client, error) {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, errors.New("Use an HTTP or HTTPS Elasticsearch address without credentials or a path.")
	}
	if !indexName.MatchString(index) {
		return nil, errors.New("Use a lowercase Elasticsearch index name with letters, digits, hyphens, or underscores.")
	}
	return &Client{
		address: strings.TrimRight(address, "/"), password: password, index: index,
		http: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

func (c *Client) EnsureIndex(ctx context.Context) error {
	body := map[string]any{
		"settings": map[string]int{"number_of_shards": 1, "number_of_replicas": 0},
		"mappings": map[string]any{
			"dynamic": "strict",
			"properties": map[string]any{
				"id":         map[string]string{"type": "keyword"},
				"content":    map[string]string{"type": "text"},
				"author":     map[string]string{"type": "keyword"},
				"created_at": map[string]string{"type": "date"},
				"updated_at": map[string]string{"type": "date"},
			},
		},
	}
	_, err := c.json(ctx, http.MethodPut, "", body)
	var upstream *APIError
	if errors.As(err, &upstream) && upstream.Status == 400 && upstream.Kind == "resource_already_exists_exception" {
		return nil
	}
	return err
}

func (c *Client) Reset(ctx context.Context) error {
	if err := c.Drop(ctx); err != nil {
		return err
	}
	return c.EnsureIndex(ctx)
}

func (c *Client) Drop(ctx context.Context) error {
	_, err := c.json(ctx, http.MethodDelete, "", nil)
	var upstream *APIError
	if errors.As(err, &upstream) && upstream.Status == 404 {
		return nil
	}
	return err
}

func (c *Client) Put(ctx context.Context, document Document) error {
	_, err := c.json(ctx, http.MethodPut, "/_doc/"+url.PathEscape(document.ID)+"?refresh=wait_for", document)
	return err
}

func (c *Client) Delete(ctx context.Context, id string) error {
	_, err := c.json(ctx, http.MethodDelete, "/_doc/"+url.PathEscape(id)+"?refresh=wait_for", nil)
	var upstream *APIError
	if errors.As(err, &upstream) && upstream.Status == 404 {
		return nil
	}
	return err
}

func (c *Client) Bulk(ctx context.Context, documents []Document) error {
	if len(documents) == 0 {
		return nil
	}
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	for _, document := range documents {
		if err := encoder.Encode(map[string]any{"index": map[string]string{"_id": document.ID}}); err != nil {
			return err
		}
		if err := encoder.Encode(document); err != nil {
			return err
		}
	}
	data, err := c.request(ctx, http.MethodPost, "/_bulk", body.Bytes(), "application/x-ndjson")
	if err != nil {
		return err
	}
	var result struct {
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Status int `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &result); err != nil || result.Errors || len(result.Items) != len(documents) {
		return ErrResponse
	}
	for _, item := range result.Items {
		if len(item) != 1 {
			return ErrResponse
		}
		for _, operation := range item {
			if operation.Status < 200 || operation.Status >= 300 {
				return ErrResponse
			}
		}
	}
	return nil
}

func (c *Client) Refresh(ctx context.Context) error {
	_, err := c.json(ctx, http.MethodPost, "/_refresh", nil)
	return err
}

func (c *Client) Find(ctx context.Context, keywords, author string, page, pageSize int) (Matches, error) {
	filter := []any{}
	must := []any{}
	if keywords != "" {
		must = append(must, map[string]any{"match": map[string]any{
			"content": map[string]string{"query": keywords, "operator": "and"},
		}})
	}
	if author != "" {
		filter = append(filter, map[string]any{"term": map[string]string{"author": author}})
	}
	offset := (page - 1) * pageSize
	body := map[string]any{
		"from": offset, "size": min(pageSize, MaxResults-offset),
		"track_total_hits": true, "_source": []string{"updated_at"},
		"query": map[string]any{"bool": map[string]any{"must": must, "filter": filter}},
		"sort":  []any{map[string]string{"_score": "desc"}, map[string]string{"created_at": "desc"}, map[string]string{"id": "desc"}},
	}
	data, err := c.json(ctx, http.MethodPost, "/_search?allow_partial_search_results=false", body)
	if err != nil {
		return Matches{}, err
	}
	var result struct {
		TimedOut bool `json:"timed_out"`
		Shards   struct {
			Failed int `json:"failed"`
		} `json:"_shards"`
		Hits *struct {
			Total *struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Items []struct {
				ID     string `json:"_id"`
				Source struct {
					UpdatedAt time.Time `json:"updated_at"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(data, &result); err != nil || result.TimedOut || result.Shards.Failed > 0 || result.Hits == nil || result.Hits.Total == nil || result.Hits.Items == nil {
		return Matches{}, ErrResponse
	}
	matches := Matches{Hits: []Hit{}, Total: result.Hits.Total.Value}
	for _, hit := range result.Hits.Items {
		matches.Hits = append(matches.Hits, Hit{ID: hit.ID, UpdatedAt: hit.Source.UpdatedAt})
	}
	return matches, nil
}

func (c *Client) json(ctx context.Context, method, path string, body any) ([]byte, error) {
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	return c.request(ctx, method, path, data, "application/json")
}

func (c *Client) request(ctx context.Context, method, path string, data []byte, contentType string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.address+"/"+c.index+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth("elastic", c.password)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err = io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 4<<20 {
		return nil, ErrResponse
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var result struct {
			Error struct {
				Kind string `json:"type"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &result)
		return nil, &APIError{Status: response.StatusCode, Kind: result.Error.Kind}
	}
	return data, nil
}
