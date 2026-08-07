package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(NewRouter())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { t.Fatalf("want 200, got %d", resp.StatusCode) }
	var body map[string]bool
	json.NewDecoder(resp.Body).Decode(&body)
	if !body["ok"] { t.Fatalf("want ok=true, got %v", body) }
}
