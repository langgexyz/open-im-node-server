package registry_test

import (
	"testing"

	"github.com/langgexyz/open-im-node-server/internal/registry"
)

func TestRegistryGetRoute(t *testing.T) {
	r := registry.New()
	r.Set("articles", "http://127.0.0.1:8081")

	got, ok := r.Get("articles")
	if !ok {
		t.Fatal("expected route to exist")
	}
	if got != "http://127.0.0.1:8081" {
		t.Fatalf("got %q", got)
	}
}

func TestRegistryGetMissing(t *testing.T) {
	r := registry.New()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("expected no route")
	}
}

func TestRegistryDelete(t *testing.T) {
	r := registry.New()
	r.Set("articles", "http://127.0.0.1:8081")
	r.Delete("articles")
	_, ok := r.Get("articles")
	if ok {
		t.Fatal("expected route to be deleted")
	}
}
