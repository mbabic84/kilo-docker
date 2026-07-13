package main

import "testing"

func TestIsSpecialNetworkModeHost(t *testing.T) {
	if !isSpecialNetworkMode("host") {
		t.Error("expected 'host' to be a special network mode")
	}
}

func TestIsSpecialNetworkModeNone(t *testing.T) {
	if !isSpecialNetworkMode("none") {
		t.Error("expected 'none' to be a special network mode")
	}
}

func TestIsSpecialNetworkModeContainer(t *testing.T) {
	if !isSpecialNetworkMode("container:my-app") {
		t.Error("expected 'container:my-app' to be a special network mode")
	}
}

func TestIsSpecialNetworkModeContainerID(t *testing.T) {
	if !isSpecialNetworkMode("container:abc123") {
		t.Error("expected 'container:abc123' to be a special network mode")
	}
}

func TestIsSpecialNetworkModeRegular(t *testing.T) {
	if isSpecialNetworkMode("my-network") {
		t.Error("expected 'my-network' to not be a special network mode")
	}
}

func TestIsSpecialNetworkModeKiloShared(t *testing.T) {
	if isSpecialNetworkMode("kilo-shared") {
		t.Error("expected 'kilo-shared' to not be a special network mode")
	}
}

func TestContainsSpecialNetworkHost(t *testing.T) {
	if !containsSpecialNetwork([]string{"kilo-shared", "host"}) {
		t.Error("expected true when host is present")
	}
}

func TestContainsSpecialNetworkNone(t *testing.T) {
	if !containsSpecialNetwork([]string{"none"}) {
		t.Error("expected true when none is present")
	}
}

func TestContainsSpecialNetworkContainer(t *testing.T) {
	if !containsSpecialNetwork([]string{"container:app"}) {
		t.Error("expected true when container: is present")
	}
}

func TestContainsSpecialNetworkFalse(t *testing.T) {
	if containsSpecialNetwork([]string{"kilo-shared", "my-net"}) {
		t.Error("expected false when no special modes are present")
	}
}

func TestContainsSpecialNetworkEmpty(t *testing.T) {
	if containsSpecialNetwork(nil) {
		t.Error("expected false for nil networks")
	}
}

func TestNormalizeNetworksHostExcludesAll(t *testing.T) {
	result := normalizeNetworks([]string{"my-net", "host", "other"}, false)
	if len(result) != 1 || result[0] != "host" {
		t.Errorf("expected [host], got %v", result)
	}
}

func TestNormalizeNetworksNoneExcludesAll(t *testing.T) {
	result := normalizeNetworks([]string{"kilo-shared", "none"}, true)
	if len(result) != 1 || result[0] != "none" {
		t.Errorf("expected [none], got %v", result)
	}
}

func TestNormalizeNetworksContainerExcludesAll(t *testing.T) {
	result := normalizeNetworks([]string{"kilo-shared", "container:app"}, true)
	if len(result) != 1 || result[0] != "container:app" {
		t.Errorf("expected [container:app], got %v", result)
	}
}

func TestNormalizeNetworksSpecialModeNotDuplicated(t *testing.T) {
	result := normalizeNetworks([]string{"host", "host"}, false)
	if len(result) != 1 || result[0] != "host" {
		t.Errorf("expected [host], got %v", result)
	}
}

func TestNormalizeNetworksRegularWithShared(t *testing.T) {
	result := normalizeNetworks([]string{"my-net"}, true)
	expected := []string{"kilo-shared", "my-net"}
	if len(result) != len(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("expected %v, got %v", expected, result)
		}
	}
}

func TestNormalizeNetworksRegularWithoutShared(t *testing.T) {
	result := normalizeNetworks([]string{"my-net"}, false)
	if len(result) != 1 || result[0] != "my-net" {
		t.Errorf("expected [my-net], got %v", result)
	}
}

func TestNormalizeNetworksEmpty(t *testing.T) {
	result := normalizeNetworks(nil, true)
	if len(result) != 1 || result[0] != "kilo-shared" {
		t.Errorf("expected [kilo-shared], got %v", result)
	}
}

func TestNormalizeNetworksDedup(t *testing.T) {
	result := normalizeNetworks([]string{"a", "b", "a"}, false)
	expected := []string{"a", "b"}
	if len(result) != len(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}
