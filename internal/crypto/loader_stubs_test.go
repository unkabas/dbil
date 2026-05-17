package crypto

import (
	"context"
	"errors"
	"testing"
)

func TestStubLoaders_AllUnavailable(t *testing.T) {
	ctx := context.Background()
	for _, l := range []Loader{NewKMSLoader(), NewKeychainLoader(), NewTTYLoader()} {
		_, _, err := l.Load(ctx)
		if !errors.Is(err, ErrLoaderUnavailable) {
			t.Errorf("%T: want ErrLoaderUnavailable, got %v", l, err)
		}
	}
}
