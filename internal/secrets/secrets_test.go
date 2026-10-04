package secrets

import "testing"

func TestMask(t *testing.T) {
	tests := []struct {
		in, want string
		n        int
	}{
		{"plain text, nothing here", "plain text, nothing here", 0},
		{"token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U ok", "token [secret] ok", 1},
		{"key sk-ant-api03-abcdefghijklmnopqrstuvwx and ghp_abcdefghijklmnopqrstuvwxyz0123456789", "key [secret] and [secret]", 2},
		{"Authorization: Bearer abcdefghijklmnopqrstuvwxyz012345", "Authorization: Bearer [secret]", 1},
		{`DB_PASSWORD="hunter2hunter2"`, `DB_PASSWORD="[secret]"`, 1},
		{"api_key: sk_live_1234567890", "api_key: [secret]", 1},
		{"mongodb+srv://admin:S3cr3tPass@cluster0.mongodb.net/app", "mongodb+srv://admin:[secret]@cluster0.mongodb.net/app", 1},
		{"max_tokens=4096 and https://example.com/path", "max_tokens=4096 and https://example.com/path", 0},
		{"api_key=sk-ant-api03-abcdefghijklmnopqrstuvwx", "api_key=[secret]", 1},
	}
	for _, tt := range tests {
		got, n := Mask(tt.in)
		if got != tt.want || n != tt.n {
			t.Errorf("Mask(%q) = (%q, %d), want (%q, %d)", tt.in, got, n, tt.want, tt.n)
		}
	}
}
