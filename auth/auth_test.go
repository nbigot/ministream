package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/nbigot/ministream/config"
	"go.uber.org/zap"
)

func TestLoadCredentialsOfTheSecuredTemplates(t *testing.T) {
	// the secrets files of the templates contain comments (keys starting with "_")
	for _, template := range []string{"secured-inmemory", "secured-jsonfilestorage"} {
		filename := filepath.Join("..", "config-templates", template, "data", "secrets", "secrets.json")
		m := AuthFile{}
		if err := m.LoadCredentialsFromFile(filename); err != nil {
			t.Fatalf("cannot load %s: %v", filename, err)
		}
		// the example passwords are the same as the user names
		if ok, err := m.AuthenticateUser("admin", "admin"); err != nil || !ok {
			t.Fatalf("%s: admin authentication failed: %v %v", template, ok, err)
		}
	}
}

func TestAuthenticateUnknownUserIsNotAnError(t *testing.T) {
	hash, err := HashPassword("sha256", 10, "salt", "secret")
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(t.TempDir(), "secrets.json")
	content := fmt.Sprintf(`{"_NOTICE": "comment", "alice": %q}`, hash)
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := AuthFile{}
	if err := m.LoadCredentialsFromFile(filename); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		user, password string
		expected       bool
	}{
		{"alice", "secret", true},
		{"alice", "wrong", false},
		{"bob", "secret", false},
	} {
		ok, err := m.AuthenticateUser(c.user, c.password)
		if err != nil || ok != c.expected {
			t.Fatalf("%s/%s: expected %v, got %v (error: %v)", c.user, c.password, c.expected, ok, err)
		}
	}
}

func TestAuthHTTPWithoutCache(t *testing.T) {
	hash, err := HashPassword("sha256", 10, "salt", "secret")
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = fmt.Fprintf(w, `{"alice": %q}`, hash)
	}))
	defer server.Close()

	c := config.AuthConfig{Enable: true, Method: "HTTP"}
	c.Methods.HTTP.Url = server.URL
	c.Methods.HTTP.Timeout = 5
	c.Methods.HTTP.CacheDurationInSec = 0
	m := AuthHTTP{}
	if err := m.Initialize(zap.NewNop(), &c); err != nil {
		t.Fatal(err)
	}

	// without cache, the credentials are loaded for every authentication
	for i := 0; i < 2; i++ {
		if ok, err := m.AuthenticateUser("alice", "secret"); err != nil || !ok {
			t.Fatalf("authentication failed: %v %v", ok, err)
		}
	}
	if ok, err := m.AuthenticateUser("bob", "secret"); err != nil || ok {
		t.Fatalf("unknown user: %v %v", ok, err)
	}
	if requests.Load() != 3 {
		t.Fatalf("expected 3 requests, got %d", requests.Load())
	}

	// with a cache, the credentials are loaded once
	c.Methods.HTTP.CacheDurationInSec = 60
	if err := m.Initialize(zap.NewNop(), &c); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if ok, err := m.AuthenticateUser("alice", "secret"); err != nil || !ok {
			t.Fatalf("authentication failed: %v %v", ok, err)
		}
	}
	if requests.Load() != 4 {
		t.Fatalf("expected 4 requests, got %d", requests.Load())
	}
}
