package llm

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"autodev/internal/core"
)

// CacheProvider wraps a Provider and caches responses to avoid redundant API calls.
type CacheProvider struct {
	inner    Provider
	mu       sync.RWMutex
	cache    map[string]*core.AgentOutput
	maxSize  int
}

// NewCacheProvider wraps a provider with in-memory response caching.
func NewCacheProvider(inner Provider, maxSize int) *CacheProvider {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &CacheProvider{
		inner:   inner,
		cache:   make(map[string]*core.AgentOutput),
		maxSize: maxSize,
	}
}

func (p *CacheProvider) Name() string {
	return p.inner.Name() + " (cached)"
}

func (p *CacheProvider) Capabilities() core.Capabilities {
	return p.inner.Capabilities()
}

func (p *CacheProvider) Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error) {
	key := cacheKey(messages, tools)

	p.mu.RLock()
	if cached, ok := p.cache[key]; ok {
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	result, err := p.inner.Chat(ctx, messages, tools)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.cache) >= p.maxSize {
		p.evict()
	}
	p.cache[key] = result
	return result, nil
}

func (p *CacheProvider) CacheSize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.cache)
}

func (p *CacheProvider) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache = make(map[string]*core.AgentOutput)
}

func (p *CacheProvider) evict() {
	for k := range p.cache {
		delete(p.cache, k)
		break
	}
}

func cacheKey(messages []core.Message, tools []core.Tool) string {
	data := struct {
		Messages []core.Message `json:"messages"`
		ToolCount int            `json:"tool_count"`
	}{Messages: messages, ToolCount: len(tools)}

	b, _ := json.Marshal(data)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}
