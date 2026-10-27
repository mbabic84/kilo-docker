package main

import "testing"

func TestNormalizeGiteaHost(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"https without trailing slash", "https://gitea.com", "https://gitea.com", false},
		{"trims whitespace and slash", "  https://gitea.com/  ", "https://gitea.com", false},
		{"http with trailing slash", "http://host/", "http://host", false},
		{"keeps path and port", "http://host:3000/gitea/", "http://host:3000/gitea", false},
		{"missing scheme", "gitea.com", "", true},
		{"rejects non-http scheme", "ftp://gitea.com", "", true},
		{"rejects scheme only", "https://", "", true},
		{"rejects userinfo", "https://user:pass@gitea.com", "", true},
		{"rejects query", "https://gitea.com?token=x", "", true},
		{"rejects fragment", "https://gitea.com#frag", "", true},
		{"rejects empty", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeGiteaHost(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeGiteaHost(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeGiteaHost(%q) error = %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("normalizeGiteaHost(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveGiteaInputsUsesStoredValues(t *testing.T) {
	t.Setenv("KD_GITEA_HOST", "")
	t.Setenv("KD_MCP_GITEA_TOKEN", "")
	t.Setenv("KD_GITEA_INSECURE", "")

	store := TokenStore{}
	store.Set(tokenKeyGiteaHost, "https://stored.example.com")
	store.Set(tokenKeyGiteaToken, "stored-token")
	store.Set(tokenKeyGiteaInsecure, "true")

	got := resolveGiteaInputs(store)

	if got.Host != "https://stored.example.com" {
		t.Errorf("Host = %q, want stored value", got.Host)
	}
	if got.Token != "stored-token" {
		t.Errorf("Token = %q, want stored value", got.Token)
	}
	if !got.Insecure {
		t.Error("Insecure = false, want true from stored value")
	}
	if got.HostFromEnv || got.TokenFromEnv || got.InsecureFromEnv {
		t.Error("empty env vars must not mark values as coming from the environment")
	}
}

func TestResolveGiteaInputsEnvShortcuts(t *testing.T) {
	t.Setenv("KD_GITEA_HOST", "https://env.example.com")
	t.Setenv("KD_MCP_GITEA_TOKEN", "env-token")
	t.Setenv("KD_GITEA_INSECURE", "true")

	store := TokenStore{}
	store.Set(tokenKeyGiteaHost, "https://stored.example.com")
	store.Set(tokenKeyGiteaToken, "stored-token")
	store.Set(tokenKeyGiteaInsecure, "")

	got := resolveGiteaInputs(store)

	if got.Host != "https://env.example.com" || !got.HostFromEnv {
		t.Errorf("Host = %q (fromEnv=%v), want env value", got.Host, got.HostFromEnv)
	}
	if got.Token != "env-token" || !got.TokenFromEnv {
		t.Errorf("Token = %q (fromEnv=%v), want env value", got.Token, got.TokenFromEnv)
	}
	if !got.Insecure || !got.InsecureFromEnv {
		t.Errorf("Insecure = %v (fromEnv=%v), want true from env", got.Insecure, got.InsecureFromEnv)
	}
}

func TestResolveGiteaInputsEnvFalseDisablesInsecure(t *testing.T) {
	t.Setenv("KD_GITEA_INSECURE", "false")

	store := TokenStore{}
	store.Set(tokenKeyGiteaInsecure, "true")

	got := resolveGiteaInputs(store)

	if got.Insecure {
		t.Error("Insecure = true, want false from env override")
	}
	if !got.InsecureFromEnv {
		t.Error("InsecureFromEnv = false, want true")
	}
}

func TestIsGiteaServiceEnabledIn(t *testing.T) {
	tests := []struct {
		services string
		want     bool
	}{
		{"gitea", true},
		{"go,gitea", true},
		{"gitea,gh", true},
		{"go,gitea,gh", true},
		{"", false},
		{"gitea-mcp", false},
		{"notgitea", false},
	}

	for _, tc := range tests {
		if got := isGiteaServiceEnabledIn(tc.services); got != tc.want {
			t.Errorf("isGiteaServiceEnabledIn(%q) = %v, want %v", tc.services, got, tc.want)
		}
	}
}
