package main

import "testing"

func TestListenAddressIsLocalOnly(t *testing.T) {
	for _, test := range []struct{ host, want string }{
		{"127.0.0.1", "127.0.0.1:8080"},
		{"127.0.0.2", "127.0.0.2:8080"},
		{"localhost", "127.0.0.1:8080"},
		{"LOCALHOST", "127.0.0.1:8080"},
		{"::1", "[::1]:8080"},
	} {
		address, err := localListenAddress(test.host, 8080)
		if err != nil || address != test.want {
			t.Fatalf("host %q: address=%q error=%v", test.host, address, err)
		}
	}
	for _, host := range []string{"", "0.0.0.0", "::", "192.168.1.10", "example.com", "127.0.0.1.example.com", "localhost.example.com"} {
		if address, err := localListenAddress(host, 8080); err == nil {
			t.Fatalf("non-local host %q accepted: %s", host, address)
		}
	}
}
