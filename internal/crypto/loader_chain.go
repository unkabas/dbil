package crypto

import (
	"context"
	"errors"
	"fmt"
)

// Chain runs Loaders in order and returns the first successful result.
// A loader returning ErrLoaderUnavailable is skipped; any other error
// from a loader stops the chain and is returned to the caller.
type Chain struct {
	Loaders []Loader
}

// NewChain returns a Chain holding the supplied loaders in priority order.
func NewChain(loaders ...Loader) *Chain {
	return &Chain{Loaders: loaders}
}

// Load tries each loader in order. Returns the first successful (key, source).
// If every loader reports ErrLoaderUnavailable, the joined chain of reasons
// is returned wrapping ErrLoaderUnavailable.
func (c *Chain) Load(ctx context.Context) (MasterKey, Source, error) {
	if len(c.Loaders) == 0 {
		return nil, "", fmt.Errorf("loader chain empty: %w", ErrLoaderUnavailable)
	}
	var reasons []error
	for _, l := range c.Loaders {
		mk, src, err := l.Load(ctx)
		if err == nil {
			return mk, src, nil
		}
		if errors.Is(err, ErrLoaderUnavailable) {
			reasons = append(reasons, err)
			continue
		}
		return nil, "", err
	}
	return nil, "", fmt.Errorf("all master key loaders unavailable: %w", errors.Join(reasons...))
}
