package storage

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalObjectStorageRoundTrip(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalObjectStorage(root)
	if err != nil {
		t.Fatal(err)
	}
	key := filepath.ToSlash(filepath.Join("questions", "42", "original.png"))
	if err := store.Put(context.Background(), key, strings.NewReader("image-bytes")); err != nil {
		t.Fatal(err)
	}
	reader, err := store.Open(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "image-bytes" {
		t.Fatalf("stored bytes = %q, want image-bytes", data)
	}
}

func TestLocalObjectStorageRejectsTraversal(t *testing.T) {
	store, err := NewLocalObjectStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "../outside", strings.NewReader("bad")); err == nil {
		t.Fatal("Put accepted a traversal key")
	}
}
