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
		{"AWS_SECRET_ACCESS_KEY=abcdEFGH1234abcdEFGH1234abcdEFGH1234abcd", "AWS_SECRET_ACCESS_KEY=[secret]", 1},
		{"aws_secret_access_key = abcdEFGH1234abcdEFGH1234abcdEFGH1234abcd", "aws_secret_access_key = [secret]", 1},
		{"SECRET_KEY = 'django-insecure-abcdef123456'", "SECRET_KEY = '[secret]'", 1},
		{"STRIPE_SECRET_KEY=" + "sk_live_" + "1234567890ab", "STRIPE_SECRET_KEY=[secret]", 1},
		{"webhook " + "whsec_" + "abcdefghijklmnopqrstuvwx", "webhook [secret]", 1},
		{"gh auth token prints " + "gho_" + "abcdefghijklmnopqrstuvwxyz0123456789", "gh auth token prints [secret]", 1},
		{"GITHUB_TOKEN=" + "ghs_" + "abcdefghijklmnopqrstuvwxyz0123456789", "GITHUB_TOKEN=[secret]", 1},
		{"TOKEN=abc123def456", "TOKEN=[secret]", 1},
		{"MAX_TOKENS=100000 and token := lexer.NextToken()", "MAX_TOKENS=100000 and token := lexer.NextToken()", 0},
		{"Authorization: Basic dXNlcjpwYXNzd29yZA==", "Authorization: Basic [secret]", 1},
		{"basic internationalization is enough", "basic internationalization is enough", 0},
		{"key " + "AIza" + "SyA1234567890abcdefghijklmnopqrstuv", "key [secret]", 1},
		{"post to https://" + "hooks.slack.com/services/" + "T00000000/B00000000/abcdefghijklmnopqrstuvwx", "post to https://hooks.slack.com/services/[secret]", 1},
		{"mysql -u root -pS3cretPass mydb", "mysql -u root -p[secret] mydb", 1},
		{"mysql -u root -p mydb", "mysql -u root -p mydb", 0},
		{"-----BEGIN " + "OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmU\nAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtz\n-----END " + "OPENSSH PRIVATE KEY-----\nnext line", "[secret]\nnext line", 1},
		{"-----BEGIN " + "RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAabcdefghij\nklmnopqrstuvwxyz0123456789", "[secret]", 1},
		{"the marker -----BEGIN " + "OPENSSH PRIVATE KEY----- (never paste one)", "the marker -----BEGIN OPENSSH PRIVATE KEY----- (never paste one)", 0},
		{"sk-ant-oat01-abcdefghijklmnopqrstuvwxyzABCDEFGH\nEf9hJAb3dEf9hJAb3dEf9hJ-xYz12345AA\nthen I pasted it", "[secret]\nthen I pasted it", 1},
	}
	for _, tt := range tests {
		got, n := Mask(tt.in)
		if got != tt.want || n != tt.n {
			t.Errorf("Mask(%q) = (%q, %d), want (%q, %d)", tt.in, got, n, tt.want, tt.n)
		}
	}
}
