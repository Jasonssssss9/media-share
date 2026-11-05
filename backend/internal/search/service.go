package search

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Jasonssssss9/media-share/backend/internal/posts"
)

type InputError string

func (e InputError) Error() string { return string(e) }

var authorName = regexp.MustCompile(`^[A-Za-z0-9_]{3,24}$`)

type Service struct {
	posts  *posts.Service
	client *Client
	writes chan struct{}
}

func NewService(postService *posts.Service, client *Client) *Service {
	return &Service{posts: postService, client: client, writes: make(chan struct{}, 1)}
}

func (s *Service) Search(ctx context.Context, keywords, author, viewerID string, page, pageSize int) (posts.Page, error) {
	keywords, author = strings.TrimSpace(keywords), strings.TrimSpace(author)
	if keywords == "" && author == "" {
		return posts.Page{}, InputError("Enter keywords or an author username.")
	}
	if utf8.RuneCountInString(keywords) > 200 || strings.ContainsRune(keywords, 0) {
		return posts.Page{}, InputError("Keep search keywords within 200 characters.")
	}
	if author != "" && !authorName.MatchString(author) {
		return posts.Page{}, InputError("Use an exact username with 3-24 letters, digits, or underscores.")
	}
	if page-1 > (MaxResults-1)/pageSize {
		return posts.Page{}, InputError("Narrow your search to view more results.")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	matches, err := s.client.Find(ctx, keywords, author, page, pageSize)
	if err != nil {
		return posts.Page{}, err
	}
	ids := make([]string, len(matches.Hits))
	revisions := make(map[string]time.Time, len(matches.Hits))
	for i, hit := range matches.Hits {
		ids[i], revisions[hit.ID] = hit.ID, hit.UpdatedAt
	}
	items, err := s.posts.GetMany(ctx, ids, viewerID)
	if err != nil {
		return posts.Page{}, err
	}
	result := posts.Page{Items: []posts.Post{}, Total: matches.Total, Page: page, PageSize: pageSize}
	for _, item := range items {
		// Stale indexed text must not make an edited or deleted post appear as a valid match.
		if !item.UpdatedAt.Equal(revisions[item.ID]) || (author != "" && item.Author.Username != author) {
			continue
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func (s *Service) Sync(ctx context.Context, id string) error {
	if err := s.lock(ctx); err != nil {
		return err
	}
	defer s.unlock()
	// Read after taking the lock so delayed requests cannot replace a newer indexed revision.
	post, err := s.posts.Get(ctx, id, "")
	if err != nil && !errors.Is(err, posts.ErrNotFound) {
		return err
	}
	if ensureErr := s.client.EnsureIndex(ctx); ensureErr != nil {
		return ensureErr
	}
	if errors.Is(err, posts.ErrNotFound) {
		return s.client.Delete(ctx, id)
	}
	return s.client.Put(ctx, document(post))
}

func (s *Service) Rebuild(ctx context.Context) (int, error) {
	if err := s.lock(ctx); err != nil {
		return 0, err
	}
	defer s.unlock()
	if err := s.client.Reset(ctx); err != nil {
		return 0, err
	}
	count := 0
	for page := 1; ; page++ {
		result, err := s.posts.List(ctx, page, 100, "", "")
		if err != nil {
			return count, err
		}
		if len(result.Items) == 0 {
			break
		}
		documents := make([]Document, len(result.Items))
		for i, post := range result.Items {
			documents[i] = document(post)
		}
		if err := s.client.Bulk(ctx, documents); err != nil {
			return count, err
		}
		count += len(documents)
	}
	return count, s.client.Refresh(ctx)
}

func (s *Service) lock(ctx context.Context) error {
	select {
	case s.writes <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) unlock() { <-s.writes }

func document(post posts.Post) Document {
	return Document{ID: post.ID, Content: post.Content, Author: post.Author.Username, CreatedAt: post.CreatedAt, UpdatedAt: post.UpdatedAt}
}
