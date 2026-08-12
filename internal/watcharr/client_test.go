package watcharr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginExchangesCredentialsForToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization header = %q", got)
		}
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Username != "philipp" || req.Password != "secret" {
			t.Fatalf("request = %+v", req)
		}
		_ = json.NewEncoder(w).Encode(LoginResponse{Token: "jwt-token"})
	}))
	defer server.Close()

	token, err := Login(context.Background(), server.URL, "philipp", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if token != "jwt-token" {
		t.Fatalf("token = %q", token)
	}
}
