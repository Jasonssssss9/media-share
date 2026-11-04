package config

import "testing"

func TestAppOrigin(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "test-only-password")
	t.Setenv("WEB_PORT", "5199")
	for _, test := range []struct {
		name   string
		origin string
		want   string
	}{
		{"follow_web_port", "", "http://localhost:5199"},
		{"explicit_origin", "https://media.example.test", "https://media.example.test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("APP_ORIGIN", test.origin)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.AppOrigin != test.want {
				t.Fatalf("origin=%q, want %q", cfg.AppOrigin, test.want)
			}
		})
	}
}
