package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"studio/internal/helpercompat"
)

func TestCommandBusHelperChecksCompatibilityBeforeSQLite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		helpercompat.Policy{Channel: helpercompat.Bus, Numbers: []int{2}}.Accept(w, r)
	}))
	defer server.Close()
	t.Setenv(helpercompat.EndpointEnv(), server.URL+helpercompat.EndpointPath)
	home := t.TempDir()
	t.Setenv(product.HomeVar(), home)
	err := runMCP(context.Background(), nil)
	if !helpercompat.IsRefusal(err) {
		t.Fatalf("runMCP: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "bus.db")); !os.IsNotExist(err) {
		t.Fatal("incompatible command helper opened SQLite")
	}
}
