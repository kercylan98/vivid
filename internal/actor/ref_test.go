package actor_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kercylan98/vivid/internal/actor"
)

func TestNewRefValid(t *testing.T) {
	tests := []struct {
		name    string
		address string
		path    string
		wantAdr string
		wantPth string
	}{
		{
			name:    "domain_without_port",
			address: "example.com",
			path:    "/",
			wantAdr: "example.com",
			wantPth: "/",
		},
		{
			name:    "domain_with_port",
			address: "example.com:8080",
			path:    "/a@b",
			wantAdr: "example.com:8080",
			wantPth: "/a@b",
		},
		{
			name:    "ipv4_with_port",
			address: "127.0.0.1:8080",
			path:    "/a/b",
			wantAdr: "127.0.0.1:8080",
			wantPth: "/a/b",
		},
		{
			name:    "ipv6_with_port",
			address: "[2001:db8::1]:443",
			path:    "/a%20b",
			wantAdr: "[2001:db8::1]:443",
			wantPth: "/a%20b",
		},
		{
			name:    "trim_spaces",
			address: " example.com ",
			path:    " /x ",
			wantAdr: "example.com",
			wantPth: "/x",
		},
		{
			name:    "localhost_without_port",
			address: "localhost",
			path:    "/local",
			wantAdr: "localhost",
			wantPth: "/local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := actor.NewRef(tt.address, tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ref.GetAddress() != tt.wantAdr {
				t.Fatalf("address mismatch, want %q got %q", tt.wantAdr, ref.GetAddress())
			}
			if ref.GetPath() != tt.wantPth {
				t.Fatalf("path mismatch, want %q got %q", tt.wantPth, ref.GetPath())
			}
		})
	}
}

func TestNewRefInvalidAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		path    string
	}{
		{name: "empty_address", address: "", path: "/"},
		{name: "ip_without_port", address: "127.0.0.1", path: "/"},
		{name: "invalid_domain", address: "-example.com", path: "/"},
		{name: "invalid_port", address: "example.com:99999", path: "/"},
		{name: "ipv6_missing_brackets", address: "2001:db8::1:80", path: "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := actor.NewRef(tt.address, tt.path)
			if ref != nil || !errors.Is(err, actor.ErrRefInvalidAddress) {
				t.Fatalf("expected ErrRefInvalidAddress, got ref=%v err=%v", ref, err)
			}
		})
	}
}

func TestNewRefInvalidPath(t *testing.T) {
	tests := []struct {
		name    string
		address string
		path    string
	}{
		{name: "empty_path", address: "example.com", path: ""},
		{name: "missing_leading_slash", address: "example.com", path: "a/b"},
		{name: "space_in_path", address: "example.com", path: "/a b"},
		{name: "invalid_percent", address: "example.com", path: "/%ZZ"},
		{name: "invalid_char", address: "example.com", path: "/a|b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := actor.NewRef(tt.address, tt.path)
			if ref != nil || err != actor.ErrRefInvalidPath {
				t.Fatalf("expected ErrRefInvalidPath, got ref=%v err=%v", ref, err)
			}
		})
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
		wantAdr string
		wantPth string
	}{
		{name: "empty", input: "", wantErr: actor.ErrRefEmpty},
		{name: "missing_path", input: "example.com", wantErr: actor.ErrRefFormat},
		{name: "invalid_path", input: "example.com/|", wantErr: actor.ErrRefInvalidPath},
		{name: "valid_domain", input: "example.com/abc@def", wantAdr: "example.com", wantPth: "/abc@def"},
		{name: "valid_localhost", input: "localhost/local", wantAdr: "localhost", wantPth: "/local"},
		{name: "valid_ipv4_slash", input: "127.0.0.1:80/a", wantAdr: "127.0.0.1:80", wantPth: "/a"},
		{name: "valid_ipv4_colon", input: "127.0.0.1:80:/a", wantAdr: "127.0.0.1:80", wantPth: "/a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := actor.ParseRef(tt.input)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				if ref != nil {
					t.Fatalf("expected nil ref, got %v", ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ref.GetAddress() != tt.wantAdr {
				t.Fatalf("address mismatch, want %q got %q", tt.wantAdr, ref.GetAddress())
			}
			if ref.GetPath() != tt.wantPth {
				t.Fatalf("path mismatch, want %q got %q", tt.wantPth, ref.GetPath())
			}
		})
	}
}

func TestNewAgentRef(t *testing.T) {
	t.Run("nil_agent", func(t *testing.T) {
		ref, err := actor.NewAgentRef(nil)
		if ref != nil || err != actor.ErrRefNilAgent {
			t.Fatalf("expected ErrRefNilAgent, got ref=%v err=%v", ref, err)
		}
	})

	t.Run("path_join", func(t *testing.T) {
		agent, err := actor.NewRef("example.com:80", "/root")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		agentRef, err := actor.NewAgentRef(agent)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(agentRef.Ref().GetPath(), "/root/@future@") {
			t.Fatalf("unexpected agent path: %q", agentRef.Ref().GetPath())
		}
	})
}

func TestRefChild(t *testing.T) {
	parent, err := actor.NewRef("example.com", "/root")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	child, err := parent.Child("sub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if child.GetPath() != "/root/sub" {
		t.Fatalf("unexpected child path: %q", child.GetPath())
	}
}

func TestRefStringFormat(t *testing.T) {
	ref, err := actor.NewRef("example.com", "/a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.String() != "example.com/a" {
		t.Fatalf("unexpected string: %q", ref.String())
	}

	ref, err = actor.NewRef("example.com:80", "/a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.String() != "example.com:80:/a" {
		t.Fatalf("unexpected string: %q", ref.String())
	}
}
