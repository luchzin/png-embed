package pngembed_test

import (
	"testing"

	"github.com/luchzin/png-embed"
)

func TestVersion(t *testing.T) {
	if pngembed.Version == "" {
		t.Fatal("expected version to be non-empty")
	}
}

