package cache

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

// Production-scale: 17k records at ~17 KB each (~289 MB total payload).
const (
	benchRecordCount = 17_000
	benchRecordBytes = 17 * 1024
)

// benchRequester simulates a real gRPC Requester: each List call returns
// freshly allocated proto objects (as a real deserializer would), so the
// old accumulate-all-then-ingest path must keep all pages alive until
// ingestItems runs, while ForEachPage can let earlier pages be GC'd.
type benchRequester struct {
	// templates holds the canonical data; List clones from these.
	templates []benchTemplate
	mu        sync.Mutex
}

type benchTemplate struct {
	id  string
	pad string
}

func newBenchRequester(count, recordBytes int) *benchRequester {
	pad := strings.Repeat("x", recordBytes)
	templates := make([]benchTemplate, count)
	for i := range count {
		templates[i] = benchTemplate{
			id:  fmt.Sprintf("agent-%06d", i),
			pad: pad,
		}
	}
	return &benchRequester{templates: templates}
}

// cloneAgent creates a fresh *v1.Agent, simulating gRPC deserialization.
func (r *benchRequester) cloneAgent(t benchTemplate) *v1.Agent {
	return &v1.Agent{
		Id:       t.id,
		Metadata: &v1.AgentMetadata{Description: t.pad},
	}
}

func (r *benchRequester) Get(_ context.Context, id string) (*v1.Agent, error) {
	for _, t := range r.templates {
		if t.id == id {
			return r.cloneAgent(t), nil
		}
	}
	return nil, ErrItemNotFound
}

func (r *benchRequester) List(_ context.Context, page *v1.PaginationRequest, _ []string) ([]*v1.Agent, *v1.PaginationResponse, error) {
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
	if end > len(r.templates) {
		end = len(r.templates)
	}

	// Allocate fresh objects per page, like a real gRPC response.
	result := make([]*v1.Agent, end-offset)
	for i, t := range r.templates[offset:end] {
		result[i] = r.cloneAgent(t)
	}

	pagination := &v1.PaginationResponse{}
	if end < len(r.templates) {
		pagination.NextToken = fmt.Sprintf("%d", end)
	}

	return result, pagination, nil
}

// BenchmarkResourceCache_FullSync_Memory measures peak heap during PerformFullSync.
// Uses UseRequester adapter with fresh allocations per page.
// Run on old and new code, then compare with: benchstat old.txt new.txt
func BenchmarkResourceCache_FullSync_Memory(b *testing.B) {
	gli := UseRequester[*v1.Agent](newBenchRequester(benchRecordCount, benchRecordBytes))

	b.ReportAllocs()
	b.ResetTimer()

	var peakHeap uint64
	for range b.N {
		cache := &ResourceCache[*v1.Agent]{
			gli:      gli,
			cache:    expirable.NewLRU[string, cacheItem[*v1.Agent]](0, nil, time.Minute),
			stopChan: make(chan struct{}),
		}

		runtime.GC()
		var before runtime.MemStats
		runtime.ReadMemStats(&before)

		if err := cache.PerformFullSync(b.Context()); err != nil {
			b.Fatal(err)
		}

		var after runtime.MemStats
		runtime.ReadMemStats(&after)

		delta := after.HeapInuse - before.HeapInuse
		if delta > peakHeap {
			peakHeap = delta
		}

		cache.Close()
	}

	b.ReportMetric(float64(peakHeap), "peak_heap_bytes/op")
}
