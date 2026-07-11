package cache

import (
	context "context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func mustHash(m proto.Message) []byte {
	h, _, err := hashMessage(m, nil)
	if err != nil {
		panic(err)
	}
	return h
}

func TestGet(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Error  string
		Result *v1.GetAuthenticatedIdentityResponse
	}
	tests := []struct {
		Name        string
		Expectation Expectation
		GLI         *testGLI
		InitCache   func(cache *expirable.LRU[string, cacheItem[*v1.GetAuthenticatedIdentityResponse]])
		Key         string
	}{
		{
			Name: "not found",
			Expectation: Expectation{
				Error: ErrItemNotFound.Error(),
			},
			Key: "unknown",
		},
		{
			Name: "cache miss",
			Expectation: Expectation{
				Error: ErrItemNotFound.Error(),
			},
			GLI: &testGLI{items: map[string]*v1.GetAuthenticatedIdentityResponse{}},
			Key: "key",
		},
		{
			Name: "other_error",
			Expectation: Expectation{
				Error: "failed to get item: some error",
			},
			GLI: &testGLI{
				getErr: fmt.Errorf("some error"),
			},
			Key: "key",
		},
		{
			Name: "cache hit",
			Expectation: Expectation{
				Result: &v1.GetAuthenticatedIdentityResponse{OrganizationId: "key"},
			},
			InitCache: func(cache *expirable.LRU[string, cacheItem[*v1.GetAuthenticatedIdentityResponse]]) {
				cache.Add("key", cacheItem[*v1.GetAuthenticatedIdentityResponse]{
					Hash:  mustHash(&v1.GetAuthenticatedIdentityResponse{OrganizationId: "key"}),
					Value: &v1.GetAuthenticatedIdentityResponse{OrganizationId: "key"},
				})
			},
			Key: "key",
		},
		{
			Name: "cache miss, data source hit",
			Expectation: Expectation{
				Result: &v1.GetAuthenticatedIdentityResponse{OrganizationId: "key"},
			},
			GLI: &testGLI{items: map[string]*v1.GetAuthenticatedIdentityResponse{
				"key": {OrganizationId: "key"},
			}},
			Key: "key",
		},
	}

	// nolint:govet
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			if test.GLI == nil {
				test.GLI = &testGLI{items: map[string]*v1.GetAuthenticatedIdentityResponse{}}
			}

			var act Expectation

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			cache, err := NewResourceCache(ctx, test.GLI, WithNoFullSync())
			if err != nil {
				t.Fatal(err)
			}
			defer cache.Close()

			if test.InitCache != nil {
				test.InitCache(cache.cache)
			}

			res, err := cache.Get(ctx, test.Key)
			if err != nil {
				act.Error = err.Error()
			}
			act.Result = res

			if diff := cmp.Diff(test.Expectation, act, protocmp.Transform()); diff != "" {
				t.Errorf("Get() mismatch (-want +got):\n%s", diff)
			}

			// Additional check for cache population on data source hit
			if test.Expectation.Error == "" && test.InitCache == nil {
				cachedItem, ok := cache.cache.Get(test.Key)
				if !ok {
					t.Errorf("Item not added to cache")
				} else if diff := cmp.Diff(test.Expectation.Result, cachedItem.Value, protocmp.Transform()); diff != "" {
					t.Errorf("Cached item mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestFullSyncLoop(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Error         string
		DidSync       bool
		Notifications []string
		Items         []string
	}
	tests := []struct {
		Name        string
		Expectation Expectation
		GLI         *testGLI
		InitCache   func(cache *expirable.LRU[string, cacheItem[*v1.GetAuthenticatedIdentityResponse]])
	}{
		{
			Name: "sync returns no items",
			GLI:  &testGLI{items: map[string]*v1.GetAuthenticatedIdentityResponse{}},
			Expectation: Expectation{
				DidSync: true,
				Items:   []string{},
			},
		},
		{
			Name: "sync does not yield different item",
			GLI: &testGLI{items: map[string]*v1.GetAuthenticatedIdentityResponse{
				"org-id": {OrganizationId: "org-id"},
			}},
			InitCache: func(cache *expirable.LRU[string, cacheItem[*v1.GetAuthenticatedIdentityResponse]]) {
				msg := &v1.GetAuthenticatedIdentityResponse{OrganizationId: "org-id"}
				cache.Add("org-id", cacheItem[*v1.GetAuthenticatedIdentityResponse]{
					Hash:  mustHash(msg),
					Value: msg,
				})
			},
			Expectation: Expectation{
				DidSync: true,
				Items:   []string{"org-id"},
			},
		},
		{
			Name: "sync yields different item",
			GLI: &testGLI{items: map[string]*v1.GetAuthenticatedIdentityResponse{
				"foobaz": {OrganizationId: "foobaz", Subject: &v1.Subject{Id: "update"}},
			}},
			InitCache: func(cache *expirable.LRU[string, cacheItem[*v1.GetAuthenticatedIdentityResponse]]) {
				msg := &v1.GetAuthenticatedIdentityResponse{OrganizationId: "foobaz", Subject: &v1.Subject{Id: "initial"}}
				cache.Add("foobaz", cacheItem[*v1.GetAuthenticatedIdentityResponse]{
					Hash:  mustHash(msg),
					Value: msg,
				})
			},
			Expectation: Expectation{
				DidSync:       true,
				Items:         []string{"foobaz"},
				Notifications: []string{"foobaz"},
			},
		},
	}

	// nolint:govet
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			var act Expectation

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			cache, err := NewResourceCache(ctx, test.GLI,
				WithItemChangedCallback(func(_ context.Context, key string) { act.Notifications = append(act.Notifications, key) }),
				WithNoFullSync(),
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { cache.Close() })

			if test.InitCache != nil {
				test.InitCache(cache.cache)
			}

			tctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
			_ = cache.PerformFullSync(tctx)
			cancel()
			waitCtx, waitCancel := context.WithTimeout(ctx, 500*time.Millisecond)
			act.DidSync = WaitForFullSynchronization(waitCtx, cache)
			waitCancel()

			act.Items = cache.cache.Keys()

			if diff := cmp.Diff(test.Expectation, act); diff != "" {
				t.Errorf("fullSyncLoop() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResourceCache_FullSync(t *testing.T) {
	t.Parallel()

	gli := &testGLI{
		items: map[string]*v1.GetAuthenticatedIdentityResponse{
			"key1": {OrganizationId: "org1"},
			"key2": {OrganizationId: "org2"},
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cache, err := NewResourceCache[*v1.GetAuthenticatedIdentityResponse](ctx, gli,
		WithFullResyncInterval(100*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()

	if !cache.WaitForFullSynchronization(ctx) {
		t.Fatal("Cache should be fully synchronized")
	}

	items, err := cache.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
}

func TestResourceCache_FullSyncAlwaysAddsItems(t *testing.T) {
	t.Parallel()

	gli := &testGLI{
		items: map[string]*v1.GetAuthenticatedIdentityResponse{
			"key1": {OrganizationId: "org1"},
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Create cache with no automatic full sync
	cache, err := NewResourceCache[*v1.GetAuthenticatedIdentityResponse](ctx, gli,
		WithNoFullSync(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()

	// TTL must be shorter than testDuration so we verify that
	// PerformFullSync refreshes expiry even when item hashes match.
	ttl := 100 * time.Millisecond
	syncInterval := 20 * time.Millisecond
	testDuration := 200 * time.Millisecond

	cache.cache = expirable.NewLRU(0, cache.onEviction, ttl)

	// Perform initial full sync to populate cache
	if err := cache.PerformFullSync(ctx); err != nil {
		t.Fatal(err)
	}

	// Verify item is in cache
	items, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("Expected item to be in cache after full sync: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}

	// Perform full syncs at regular intervals for longer than the TTL
	// This tests that ingestItems always calls Add() to refresh expiry,
	// even when the item hash matches (unchanged item)
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	deadline := time.Now().Add(testDuration)
	syncCount := 0
	for time.Now().Before(deadline) {
		<-ticker.C
		syncCount++

		// Perform full sync with unchanged item
		if err := cache.PerformFullSync(ctx); err != nil {
			t.Fatalf("Full sync failed at iteration %d: %v", syncCount, err)
		}

		// Verify item is still in cache
		items, err := cache.List(ctx)
		if err != nil {
			t.Fatalf("Failed to list items at iteration %d: %v", syncCount, err)
		}
		if len(items) != 1 {
			t.Fatalf("Expected 1 item after sync %d (elapsed: %v), got %d items",
				syncCount, time.Since(deadline.Add(-testDuration)), len(items))
		}
	}

	t.Logf("Successfully performed %d syncs over %v with TTL of %v", syncCount, testDuration, ttl)
}

func TestResourceCache_FullSync_ForEachPage(t *testing.T) {
	t.Parallel()

	gli := &testGLI{
		items: map[string]*v1.GetAuthenticatedIdentityResponse{
			"org1": {OrganizationId: "org1"},
			"org2": {OrganizationId: "org2"},
			"org3": {OrganizationId: "org3"},
		},
	}

	cache := &ResourceCache[*v1.GetAuthenticatedIdentityResponse]{
		gli:      gli,
		cache:    expirable.NewLRU[string, cacheItem[*v1.GetAuthenticatedIdentityResponse]](0, nil, time.Minute),
		stopChan: make(chan struct{}),
	}
	defer cache.Close()

	if err := cache.PerformFullSync(t.Context()); err != nil {
		t.Fatal(err)
	}

	if !cache.FullySynchronized() {
		t.Error("Cache should be fully synchronized after PerformFullSync")
	}

	items, err := cache.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(items))
	}
}

func TestResourceCache_FullSync_ForEachPageError(t *testing.T) {
	t.Parallel()

	gli := &testGLI{
		items: map[string]*v1.GetAuthenticatedIdentityResponse{
			"org1": {OrganizationId: "org1"},
		},
		listErr: fmt.Errorf("page fetch failed"),
	}

	cache := &ResourceCache[*v1.GetAuthenticatedIdentityResponse]{
		gli:      gli,
		cache:    expirable.NewLRU[string, cacheItem[*v1.GetAuthenticatedIdentityResponse]](0, nil, time.Minute),
		stopChan: make(chan struct{}),
	}
	defer cache.Close()

	err := cache.PerformFullSync(t.Context())
	if err == nil {
		t.Fatal("Expected error from PerformFullSync")
	}
	if !errors.Is(err, ErrSyncFailed) {
		t.Errorf("Expected ErrSyncFailed, got: %v", err)
	}

	if cache.FullySynchronized() {
		t.Error("Cache should NOT be fully synchronized after failed sync")
	}
}

func TestResourceCache_RemoveItem(t *testing.T) {
	t.Parallel()

	gli := &testGLI{
		items: map[string]*v1.GetAuthenticatedIdentityResponse{
			"key1": {OrganizationId: "org1"},
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	removedKeyCh := make(chan string, 1)
	cache, err := NewResourceCache[*v1.GetAuthenticatedIdentityResponse](ctx, gli,
		WithItemChangedCallback(func(ctx context.Context, key string) {
			select {
			case removedKeyCh <- key:
			default:
			}
		}),
		WithNoFullSync(),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cache.Close()
	})

	// Add item to cache
	_, err = cache.Get(t.Context(), "key1")
	if err != nil {
		t.Fatal(err)
	}

	// Remove item
	cache.RemoveItem(t.Context(), "key1")

	// Wait for the item changed callback
	select {
	case removedKey := <-removedKeyCh:
		if removedKey != "key1" {
			t.Errorf("Expected invalidated key to be 'key1', got '%s'", removedKey)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for item changed callback")
	}

	// Check that item is no longer in cache
	items, err := cache.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 0 {
		t.Errorf("Expected 0 items after invalidation, got %d", len(items))
	}
}

func TestInvalidateItem(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Error         string
		Notifications []string
		Items         []string
	}
	tests := []struct {
		Name             string
		Expectation      Expectation
		GLI              *testGLI
		Action           func(cache *ResourceCache[*v1.GetAuthenticatedIdentityResponse]) []*v1.GetAuthenticatedIdentityResponse
		PostInvalidation func(*testGLI)
	}{
		{
			Name: "get post invalidation",
			GLI: &testGLI{
				items: map[string]*v1.GetAuthenticatedIdentityResponse{
					"key1": {OrganizationId: "key1"},
				},
			},
			PostInvalidation: func(gli *testGLI) {
				gli.items = map[string]*v1.GetAuthenticatedIdentityResponse{
					"key1": {OrganizationId: "org1"},
				}
			},
			Action: func(cache *ResourceCache[*v1.GetAuthenticatedIdentityResponse]) []*v1.GetAuthenticatedIdentityResponse {
				r, _ := cache.Get(t.Context(), "key1")
				return []*v1.GetAuthenticatedIdentityResponse{r}
			},
			Expectation: Expectation{
				Notifications: []string{"key1"},
				Items:         []string{"org1"},
			},
		},
		{
			Name: "get removed post invalidation",
			GLI: &testGLI{
				items: map[string]*v1.GetAuthenticatedIdentityResponse{
					"key1": {OrganizationId: "key1"},
				},
			},
			PostInvalidation: func(gli *testGLI) {
				gli.items = map[string]*v1.GetAuthenticatedIdentityResponse{}
			},
			Action: func(cache *ResourceCache[*v1.GetAuthenticatedIdentityResponse]) []*v1.GetAuthenticatedIdentityResponse {
				r, _ := cache.Get(t.Context(), "key1")
				if r == nil {
					return nil
				}
				return []*v1.GetAuthenticatedIdentityResponse{r}
			},
			Expectation: Expectation{
				Notifications: []string{"key1"},
			},
		},
		{
			Name: "list post invalidation",
			GLI: &testGLI{
				items: map[string]*v1.GetAuthenticatedIdentityResponse{
					"key1": {OrganizationId: "key1"},
				},
			},
			Action: func(cache *ResourceCache[*v1.GetAuthenticatedIdentityResponse]) []*v1.GetAuthenticatedIdentityResponse {
				r, _ := cache.List(t.Context())
				return r
			},
			Expectation: Expectation{
				Notifications: []string{"key1"},
				Items:         []string{"key1"},
			},
		},
		{
			Name: "list removed post invalidation",
			GLI: &testGLI{
				items: map[string]*v1.GetAuthenticatedIdentityResponse{
					"key1": {OrganizationId: "key1"},
				},
			},
			PostInvalidation: func(gli *testGLI) {
				gli.items = map[string]*v1.GetAuthenticatedIdentityResponse{}
			},
			Action: func(cache *ResourceCache[*v1.GetAuthenticatedIdentityResponse]) []*v1.GetAuthenticatedIdentityResponse {
				r, _ := cache.List(t.Context())
				return r
			},
			Expectation: Expectation{
				Notifications: []string{"key1"},
			},
		},
		{
			Name: "new item",
			GLI:  &testGLI{},
			PostInvalidation: func(gli *testGLI) {
				gli.items = map[string]*v1.GetAuthenticatedIdentityResponse{
					"key1": {OrganizationId: "key1"},
				}
			},
			Action: func(cache *ResourceCache[*v1.GetAuthenticatedIdentityResponse]) []*v1.GetAuthenticatedIdentityResponse {
				r, _ := cache.Get(t.Context(), "key1")
				return []*v1.GetAuthenticatedIdentityResponse{r}
			},
			Expectation: Expectation{
				Notifications: []string{"key1"},
				Items:         []string{"key1"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			var act Expectation

			gli := test.GLI
			if gli == nil {
				gli = &testGLI{}
			}

			ctx := t.Context()
			cache, err := NewResourceCache[*v1.GetAuthenticatedIdentityResponse](ctx, gli,
				WithItemChangedCallback(func(ctx context.Context, key string) {
					act.Notifications = append(act.Notifications, key)
				}),
				WithFullResyncInterval(0),
				WithMaxEntries(100),
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { cache.Close() })
			for key, item := range gli.items {
				cache.cache.Add(key, cacheItem[*v1.GetAuthenticatedIdentityResponse]{
					Hash:  mustHash(item),
					Value: item,
				})
			}

			// invalidate item
			cache.InvalidateItem(t.Context(), "key1")
			if test.PostInvalidation != nil {
				test.PostInvalidation(gli)
			}

			rawItems := test.Action(cache)
			for _, item := range rawItems {
				act.Items = append(act.Items, item.OrganizationId)
			}

			if diff := cmp.Diff(test.Expectation, act); diff != "" {
				t.Errorf("InvalidateItem() mismatch (-want  got):\n%s", diff)
			}
		})
	}
}

func TestInvalidateItem_CleansUpPhantomEntries(t *testing.T) {
	t.Parallel()

	gli := &testGLI{
		items: map[string]*v1.GetAuthenticatedIdentityResponse{
			"svc-1": {OrganizationId: "svc-1"},
		},
	}

	cache, err := NewResourceCache[*v1.GetAuthenticatedIdentityResponse](t.Context(), gli,
		WithFullResyncInterval(0),
		WithMaxEntries(100),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cache.Close() })

	// Seed the cache with the known item.
	cache.cache.Add("svc-1", cacheItem[*v1.GetAuthenticatedIdentityResponse]{
		Hash:  mustHash(gli.items["svc-1"]),
		Value: gli.items["svc-1"],
	})

	// Invalidate an ID that doesn't belong to this cache (e.g., an environment ID
	// broadcast to a ServiceCache). This creates a phantom zero-value entry.
	cache.InvalidateItem(t.Context(), "env-unknown")

	// First List call: sees the phantom entry, calls Refetch for it, gets nothing back,
	// and removes the phantom entry from the cache.
	items, err := cache.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].OrganizationId != "svc-1" {
		t.Errorf("expected [svc-1], got %v", items)
	}

	gli.mu.Lock()
	listCountAfterFirst := gli.listCount
	gli.mu.Unlock()

	// Second List call: the phantom entry was removed, so no Refetch is needed.
	items, err = cache.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	gli.mu.Lock()
	listCountAfterSecond := gli.listCount
	gli.mu.Unlock()

	if len(items) != 1 || items[0].OrganizationId != "svc-1" {
		t.Errorf("expected [svc-1], got %v", items)
	}
	if listCountAfterSecond != listCountAfterFirst {
		t.Errorf("expected no additional List API calls after phantom cleanup, but got %d", listCountAfterSecond-listCountAfterFirst)
	}
}

func TestResourceCache_Close(t *testing.T) {
	t.Parallel()

	gli := &testGLI{
		items: map[string]*v1.GetAuthenticatedIdentityResponse{
			"key1": {OrganizationId: "org1"},
		},
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cache, err := NewResourceCache[*v1.GetAuthenticatedIdentityResponse](ctx, gli)
	if err != nil {
		t.Fatal(err)
	}

	// Add item to cache
	_, err = cache.Get(t.Context(), "key1")
	if err != nil {
		t.Fatal(err)
	}

	// Close the cache
	err = cache.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to get item after close
	_, err = cache.Get(t.Context(), "key1")
	if !errors.Is(err, ErrCacheClosed) {
		t.Errorf("Expected ErrCacheClosed, got %v", err)
	}

	// Attempt to list items after close
	_, err = cache.List(t.Context())
	if !errors.Is(err, ErrCacheClosed) {
		t.Errorf("Expected ErrCacheClosed, got %v", err)
	}
}

type testGLI struct {
	items     map[string]*v1.GetAuthenticatedIdentityResponse
	getCount  int
	listCount int
	mu        sync.Mutex
	getErr    error
	listErr   error
}

func (t *testGLI) Get(ctx context.Context, key string) (*v1.GetAuthenticatedIdentityResponse, error) {
	if t != nil && t.getErr != nil {
		return nil, t.getErr
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.getCount++
	res, ok := t.items[key]
	if !ok {
		return nil, ErrItemNotFound
	}
	return res, nil
}

func (t *testGLI) List(ctx context.Context, maxItems int, ids []string) ([]*v1.GetAuthenticatedIdentityResponse, error) {
	if t != nil && t.listErr != nil {
		return nil, t.listErr
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.listCount++
	res := make([]*v1.GetAuthenticatedIdentityResponse, 0, len(t.items))

	if len(ids) > 0 {
		for _, id := range ids {
			item, ok := t.items[id]
			if ok {
				res = append(res, item)
			}
			if maxItems != 0 && len(res) == maxItems {
				break
			}
		}
	} else {
		for _, item := range t.items {
			res = append(res, item)
			if maxItems != 0 && len(res) == maxItems {
				break
			}
		}
	}
	return res, nil
}

func (t *testGLI) ForEachPage(_ context.Context, fn func(page []*v1.GetAuthenticatedIdentityResponse) error) error {
	if t != nil && t.listErr != nil {
		return t.listErr
	}

	items := make([]*v1.GetAuthenticatedIdentityResponse, 0, len(t.items))
	for _, item := range t.items {
		items = append(items, item)
	}
	return fn(items)
}

func (t *testGLI) Index(item *v1.GetAuthenticatedIdentityResponse) string {
	return item.OrganizationId
}

// testRequester implements Requester with pagination support for testing ForEachPage.
type testRequester struct {
	sorted []*v1.Agent
	mu     sync.Mutex
}

// newTestRequester creates a testRequester with count items.
// recordBytes controls the approximate size of each Agent proto.
// Pass 0 for minimal-size records.
func newTestRequester(count, recordBytes int) *testRequester {
	sorted := make([]*v1.Agent, count)
	for i := range count {
		agent := &v1.Agent{Id: fmt.Sprintf("agent%d", i)}
		if recordBytes > 0 {
			agent.Metadata = &v1.AgentMetadata{
				Description: strings.Repeat("x", recordBytes),
			}
		}
		sorted[i] = agent
	}
	return &testRequester{sorted: sorted}
}

func (r *testRequester) Get(_ context.Context, id string) (*v1.Agent, error) {
	for _, item := range r.sorted {
		if item.Id == id {
			return item, nil
		}
	}
	return nil, ErrItemNotFound
}

func (r *testRequester) List(_ context.Context, page *v1.PaginationRequest, _ []string) ([]*v1.Agent, *v1.PaginationResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	offset := 0
	if page != nil && page.Token != "" {
		fmt.Sscanf(page.Token, "%d", &offset)
	}

	pageSize := 100
	if page != nil && page.PageSize > 0 {
		pageSize = int(page.PageSize)
	}

	end := offset + pageSize
	if end > len(r.sorted) {
		end = len(r.sorted)
	}

	result := r.sorted[offset:end]

	pagination := &v1.PaginationResponse{}
	if end < len(r.sorted) {
		pagination.NextToken = fmt.Sprintf("%d", end)
	}

	return result, pagination, nil
}

func TestHashMessage(t *testing.T) {
	t.Parallel()

	large := &v1.Agent{
		Id:       "large",
		Metadata: &v1.AgentMetadata{Description: strings.Repeat("x", 20_000)},
	}
	small := &v1.Agent{
		Id: "small",
	}

	t.Run("nil buffer", func(t *testing.T) {
		t.Parallel()

		h1, _, err := hashMessage(small, nil)
		if err != nil {
			t.Fatal(err)
		}
		h2, _, err := hashMessage(small, nil)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(h1, h2); diff != "" {
			t.Errorf("same message with nil buffer produced different hashes (-first +second):\n%s", diff)
		}
	})

	t.Run("buffer reuse large then small", func(t *testing.T) {
		t.Parallel()

		// Reference hashes with fresh buffers.
		largeRef, _, err := hashMessage(large, nil)
		if err != nil {
			t.Fatal(err)
		}
		smallRef, _, err := hashMessage(small, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Hash large then small with buffer reuse — the small hash must not
		// contain leftover bytes from the large message's serialization.
		var buf []byte
		gotLarge, buf, err := hashMessage(large, buf)
		if err != nil {
			t.Fatal(err)
		}
		gotSmall, _, err := hashMessage(small, buf)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(largeRef, gotLarge); diff != "" {
			t.Errorf("large hash mismatch with buffer reuse (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(smallRef, gotSmall); diff != "" {
			t.Errorf("small hash mismatch with buffer reuse (-want +got):\n%s", diff)
		}
	})
}

func TestGetListIndexAdapter_ForEachPage(t *testing.T) {
	t.Parallel()

	requester := newTestRequester(250, 0)
	adapter := UseRequester[*v1.Agent](requester)

	var pages []int
	var allItems []*v1.Agent
	err := adapter.ForEachPage(t.Context(), func(page []*v1.Agent) error {
		pages = append(pages, len(page))
		allItems = append(allItems, page...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(allItems) != 250 {
		t.Errorf("Expected 250 items total, got %d", len(allItems))
	}

	// 250 items at page size 100 = 3 pages (100 + 100 + 50).
	if len(pages) != 3 {
		t.Errorf("Expected 3 pages, got %d: %v", len(pages), pages)
	}
}

func TestGetListIndexAdapter_ForEachPage_CallbackError(t *testing.T) {
	t.Parallel()

	requester := newTestRequester(50, 0)
	adapter := UseRequester[*v1.Agent](requester)

	callbackErr := fmt.Errorf("callback failed")
	err := adapter.ForEachPage(t.Context(), func(page []*v1.Agent) error {
		return callbackErr
	})

	if !errors.Is(err, callbackErr) {
		t.Errorf("Expected callback error, got: %v", err)
	}
}

// performanceBudget defines the performance expectations for a benchmark
type performanceBudget struct {
	opsPerSecond float64
	allocsPerOp  int64
	bytesPerOp   int64
}

// checkPerformanceBudget checks if the benchmark results are within the defined budget
func checkPerformanceBudget(b *testing.B, budget performanceBudget) {
	b.Helper()
	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			// Run the benchmark function here
			b.StopTimer()
			b.StartTimer()
		}
	})

	opsPerSecond := float64(result.N) / result.T.Seconds()
	if opsPerSecond < budget.opsPerSecond {
		b.Fatalf("Performance budget exceeded: got %.2f ops/sec, want at least %.2f ops/sec", opsPerSecond, budget.opsPerSecond)
	}
	if result.AllocsPerOp() > budget.allocsPerOp {
		b.Fatalf("Allocation budget exceeded: got %d allocs/op, want at most %d allocs/op", result.AllocsPerOp(), budget.allocsPerOp)
	}
	if result.AllocedBytesPerOp() > budget.bytesPerOp {
		b.Fatalf("Allocation budget exceeded: got %d bytes/op, want at most %d bytes/op", result.AllocedBytesPerOp(), budget.bytesPerOp)
	}
}

// Run benchmarks running:
// go test -benchmem -run='^$' -bench '^(BenchmarkResourceCache_Get|BenchmarkResourceCache_List|BenchmarkResourceCache_FullSync|BenchmarkResourceCache_Concurrent|BenchmarkWithBudgets)$' github.com/gitpod-io/terraform-provider-ona/internal/api/go/client/cache
func BenchmarkResourceCache_Get(b *testing.B) {
	gli := &testGLI{
		items: make(map[string]*v1.GetAuthenticatedIdentityResponse),
	}
	for i := range 1000 {
		key := fmt.Sprintf("key-%d", i)
		gli.items[key] = &v1.GetAuthenticatedIdentityResponse{OrganizationId: key}
	}

	ctx, cancel := context.WithCancel(b.Context())
	defer cancel()

	cache, err := NewResourceCache(ctx, gli)
	if err != nil {
		b.Fatal(err)
	}
	defer cache.Close()

	b.ResetTimer()
	for i := range b.N {
		key := fmt.Sprintf("key-%d", i%1000)
		_, err := cache.Get(b.Context(), key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResourceCache_List(b *testing.B) {
	gli := &testGLI{
		items: make(map[string]*v1.GetAuthenticatedIdentityResponse),
	}
	for i := range 1000 {
		key := fmt.Sprintf("key-%d", i)
		gli.items[key] = &v1.GetAuthenticatedIdentityResponse{OrganizationId: key}
	}

	ctx, cancel := context.WithCancel(b.Context())
	defer cancel()

	cache, err := NewResourceCache(ctx, gli)
	if err != nil {
		b.Fatal(err)
	}
	defer cache.Close()

	// Populate the cache
	for key := range gli.items {
		_, err := cache.Get(b.Context(), key)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for range b.N {
		_, err := cache.List(b.Context())
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResourceCache_FullSync(b *testing.B) {
	gli := &testGLI{
		items: make(map[string]*v1.GetAuthenticatedIdentityResponse),
	}
	for i := range 1000 {
		key := fmt.Sprintf("key-%d", i)
		gli.items[key] = &v1.GetAuthenticatedIdentityResponse{OrganizationId: key}
	}

	ctx, cancel := context.WithTimeout(b.Context(), 10*time.Second)
	defer cancel()

	b.ResetTimer()
	for range b.N {
		cache, err := NewResourceCache(ctx, gli, WithFullResyncInterval(time.Millisecond))
		if err != nil {
			b.Fatal(err)
		}

		// Wait for the first sync to complete
		didSync := WaitForFullSynchronization(ctx, cache)
		if !didSync {
			b.Fatal("Cache did not fully sync")
		}

		cache.Close()
	}
}

func BenchmarkResourceCache_Concurrent(b *testing.B) {
	gli := &testGLI{
		items: make(map[string]*v1.GetAuthenticatedIdentityResponse),
	}
	for i := range 1000 {
		key := fmt.Sprintf("key-%d", i)
		gli.items[key] = &v1.GetAuthenticatedIdentityResponse{OrganizationId: key}
	}

	ctx, cancel := context.WithCancel(b.Context())
	defer cancel()

	cache, err := NewResourceCache(ctx, gli)
	if err != nil {
		b.Fatal(err)
	}
	defer cache.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%1000)
			_, err := cache.Get(b.Context(), key)
			if err != nil {
				b.Error(err)
			}
			i++
		}
	})
}

// Add this function to run all benchmarks with performance budgets
func BenchmarkWithBudgets(b *testing.B) {
	benchmarks := []struct {
		name   string
		bench  func(*testing.B)
		budget performanceBudget
	}{
		{"ResourceCache_Get", BenchmarkResourceCache_Get, performanceBudget{opsPerSecond: 100000, allocsPerOp: 2, bytesPerOp: 64}},
		{"ResourceCache_List", BenchmarkResourceCache_List, performanceBudget{opsPerSecond: 10000, allocsPerOp: 5, bytesPerOp: 1024}},
		{"ResourceCache_FullSync", BenchmarkResourceCache_FullSync, performanceBudget{opsPerSecond: 100, allocsPerOp: 10000, bytesPerOp: 1e6}},
		{"ResourceCache_Concurrent", BenchmarkResourceCache_Concurrent, performanceBudget{opsPerSecond: 500000, allocsPerOp: 2, bytesPerOp: 64}},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			result := testing.Benchmark(bm.bench)
			checkPerformanceBudget(b, bm.budget)
			b.Logf("%s: %s", bm.name, result.String())
		})
	}
}

func TestRefetch(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Error  string
		Result map[string]*v1.GetAuthenticatedIdentityResponse
	}
	tests := []struct {
		Name        string
		Expectation Expectation
		GLI         *testGLI
		InitCache   func(cache *expirable.LRU[string, cacheItem[*v1.GetAuthenticatedIdentityResponse]])
		Keys        []string
	}{
		{
			Name: "empty refetch",
			GLI:  &testGLI{},
			Keys: []string{},
		},
		{
			Name: "successful refetch",
			Expectation: Expectation{
				Result: map[string]*v1.GetAuthenticatedIdentityResponse{
					"key1": {OrganizationId: "key1"},
					"key2": {OrganizationId: "key2"},
				},
			},
			GLI: &testGLI{items: map[string]*v1.GetAuthenticatedIdentityResponse{
				"key1": {OrganizationId: "key1"},
				"key2": {OrganizationId: "key2"},
			}},
			Keys: []string{"key1", "key2"},
		},
		{
			Name: "refetch with GLI error",
			Expectation: Expectation{
				Error: "failed to refetch items: GLI error",
			},
			GLI:  &testGLI{listErr: errors.New("GLI error")},
			Keys: []string{"key1", "key2"},
		},
		{
			Name: "refetch more than 25 keys",
			Expectation: Expectation{
				Result: map[string]*v1.GetAuthenticatedIdentityResponse{
					"key1": {OrganizationId: "key1"},
					"key2": {OrganizationId: "key2"},
				},
			},
			GLI: &testGLI{items: map[string]*v1.GetAuthenticatedIdentityResponse{
				"key1": {OrganizationId: "key1"},
				"key2": {OrganizationId: "key2"},
			}},
			Keys: func() []string {
				keys := make([]string, 26)
				for i := range keys {
					keys[i] = fmt.Sprintf("key%d", i)
				}
				return keys
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			var act Expectation

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			cache, err := NewResourceCache(ctx, test.GLI, WithNoFullSync())
			if err != nil {
				t.Fatal(err)
			}
			defer cache.Close()

			if test.InitCache != nil {
				test.InitCache(cache.cache)
			}

			err = cache.Refetch(ctx, test.Keys)
			if err != nil {
				act.Error = err.Error()
			}

			act.Result = make(map[string]*v1.GetAuthenticatedIdentityResponse)
			for _, key := range test.Keys {
				if item, ok := cache.cache.Get(key); ok {
					act.Result[key] = item.Value
				}
			}
			if len(act.Result) == 0 {
				act.Result = nil
			}

			if diff := cmp.Diff(test.Expectation, act, protocmp.Transform()); diff != "" {
				t.Errorf("Refetch() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFullSyncLoop_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Test that PerformFullSync properly wraps context.Canceled
	cancelledCtx, cancelFunc := context.WithCancel(t.Context())
	cancelFunc() // Cancel immediately

	gli := &testGLI{
		listErr: context.Canceled,
	}

	ctx := t.Context()
	cache, err := NewResourceCache(ctx, gli, WithNoFullSync())
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()

	err = cache.PerformFullSync(cancelledCtx)
	if err == nil {
		t.Fatal("Expected error from PerformFullSync with cancelled context")
	}

	// Verify that the error can be unwrapped to context.Canceled
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected error to wrap context.Canceled, got: %v", err)
	}

	// Verify that the error also wraps ErrSyncFailed
	if !errors.Is(err, ErrSyncFailed) {
		t.Errorf("Expected error to wrap ErrSyncFailed, got: %v", err)
	}
}

type listAttempt struct {
	items []*v1.GetAuthenticatedIdentityResponse
	err   error
}

type retryTestGLI struct {
	listAttempts []listAttempt
	listCount    int
	mu           sync.Mutex
}

func (t *retryTestGLI) Get(ctx context.Context, key string) (*v1.GetAuthenticatedIdentityResponse, error) {
	return nil, ErrItemNotFound
}

func (t *retryTestGLI) List(ctx context.Context, maxItems int, ids []string) ([]*v1.GetAuthenticatedIdentityResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.listCount < len(t.listAttempts) {
		attempt := t.listAttempts[t.listCount]
		t.listCount++
		return attempt.items, attempt.err
	}

	if len(t.listAttempts) > 0 {
		lastAttempt := t.listAttempts[len(t.listAttempts)-1]
		return lastAttempt.items, lastAttempt.err
	}

	return nil, errors.New("no attempts configured")
}

func (t *retryTestGLI) ForEachPage(ctx context.Context, fn func(page []*v1.GetAuthenticatedIdentityResponse) error) error {
	items, err := t.List(ctx, 0, nil)
	if err != nil {
		return err
	}
	return fn(items)
}

func (t *retryTestGLI) Index(item *v1.GetAuthenticatedIdentityResponse) string {
	return item.OrganizationId
}

func TestPerformFullSyncWithRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupGLI     func() GetListIndex[*v1.GetAuthenticatedIdentityResponse]
		setupContext func() (context.Context, context.CancelFunc)
		timeout      time.Duration
		err          string
	}{
		{
			name: "context cancellation",
			setupGLI: func() GetListIndex[*v1.GetAuthenticatedIdentityResponse] {
				return &testGLI{
					items:   map[string]*v1.GetAuthenticatedIdentityResponse{},
					listErr: context.Canceled,
				}
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			timeout: 100 * time.Millisecond,
			err:     "full sync operation failed: context canceled",
		},
		{
			name: "permanent error - no retries",
			setupGLI: func() GetListIndex[*v1.GetAuthenticatedIdentityResponse] {
				return &testGLI{
					items:   map[string]*v1.GetAuthenticatedIdentityResponse{},
					listErr: connect.NewError(connect.CodeInvalidArgument, errors.New("invalid request")),
				}
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			timeout: 1 * time.Second,
			err:     "full sync operation failed: invalid_argument: invalid request",
		},
		{
			name: "transient error - retries until timeout",
			setupGLI: func() GetListIndex[*v1.GetAuthenticatedIdentityResponse] {
				return &testGLI{
					items:   map[string]*v1.GetAuthenticatedIdentityResponse{},
					listErr: connect.NewError(connect.CodeUnavailable, errors.New("service unavailable")),
				}
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			timeout: 200 * time.Millisecond,
			err:     "context deadline exceeded",
		},
		{
			name: "transient error - succeeds after retries",
			setupGLI: func() GetListIndex[*v1.GetAuthenticatedIdentityResponse] {
				return &retryTestGLI{
					listAttempts: []listAttempt{
						{
							items: nil,
							err:   connect.NewError(connect.CodeUnavailable, errors.New("service unavailable")),
						},
						{
							items: nil,
							err:   connect.NewError(connect.CodeUnavailable, errors.New("service unavailable")),
						},
						{
							items: []*v1.GetAuthenticatedIdentityResponse{{OrganizationId: "test"}},
							err:   nil,
						},
					},
				}
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			timeout: 5 * time.Second,
			err:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := tt.setupContext()
			defer cancel()

			gli := tt.setupGLI()
			cache := &ResourceCache[*v1.GetAuthenticatedIdentityResponse]{
				gli:   gli,
				cache: expirable.NewLRU[string, cacheItem[*v1.GetAuthenticatedIdentityResponse]](0, nil, time.Minute),
			}

			err := cache.performFullSyncWithRetry(ctx, tt.timeout)

			var gotErr string
			if err != nil {
				gotErr = err.Error()
			}

			if diff := cmp.Diff(tt.err, gotErr); diff != "" {
				t.Errorf("performFullSyncWithRetry() error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
