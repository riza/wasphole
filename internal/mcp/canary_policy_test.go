package mcp

import "testing"

func TestShouldInjectCredentialForPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "passwd is not a secret insertion point", path: "/etc/passwd", want: false},
		{name: "shadow permission errors are not decorated", path: "/etc/shadow", want: false},
		{name: "proc environ naturally carries env secrets", path: "/proc/self/environ", want: true},
		{name: "dotenv naturally carries secrets", path: "/app/.env", want: true},
		{name: "config can carry app credentials", path: "/etc/config.yaml", want: true},
		{name: "private key path carries credentials", path: "/home/app/.ssh/id_rsa", want: true},
		{name: "logs should stay clean", path: "/var/log/auth.log", want: false},
		{name: "ordinary proc output should stay clean", path: "/proc/net/tcp", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldInjectCredentialForPath(tt.path); got != tt.want {
				t.Fatalf("shouldInjectCredentialForPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
