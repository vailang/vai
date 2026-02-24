package runner

import (
	"sync"

	"github.com/vailang/vai/internal/coder"
)

// coderPool manages reusable Coder instances keyed by absolute file path.
// Each coder holds a tree-sitter parser that is expensive to create.
// Reusing the parser avoids repeated allocation during diff/execute/debug steps.
type coderPool struct {
	mu     sync.Mutex
	coders map[string]*coder.Coder
}

func newCoderPool() *coderPool {
	return &coderPool{coders: make(map[string]*coder.Coder)}
}

// Get returns a coder for the given path, creating one if needed.
// The returned coder is loaded with the provided content.
// The caller must NOT call Close on the returned coder — the pool owns the lifecycle.
func (p *coderPool) Get(absPath string, content []byte) (*coder.Coder, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if c, ok := p.coders[absPath]; ok {
		// Re-load with current content to refresh symbols.
		if err := c.Load(content); err != nil {
			return nil, err
		}
		return c, nil
	}

	lang, err := coder.DetectLanguage(absPath)
	if err != nil {
		return nil, err
	}
	c, err := coder.New(lang, absPath)
	if err != nil {
		return nil, err
	}
	if err := c.Load(content); err != nil {
		c.Close()
		return nil, err
	}
	p.coders[absPath] = c
	return c, nil
}

// Close releases all tree-sitter resources.
func (p *coderPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.coders {
		c.Close()
	}
	p.coders = nil
}
