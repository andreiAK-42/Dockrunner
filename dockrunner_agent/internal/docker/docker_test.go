package docker

import "testing"

func TestBuildArgsFromEnv(t *testing.T) {
	if got := buildArgsFromEnv(nil); got != nil {
		t.Fatalf("buildArgsFromEnv(nil) = %v, want nil", got)
	}

	args := buildArgsFromEnv(map[string]string{
		"DATABASE_URL": "postgres://db/app",
		"EMPTY":        "",
		"_PRIVATE":     "1",
		"bad-name":     "skip",
		"1NUMBER":      "skip",
		"":             "skip",
	})
	want := map[string]string{
		"DATABASE_URL": "postgres://db/app",
		"EMPTY":        "",
		"_PRIVATE":     "1",
	}
	if len(args) != len(want) {
		t.Fatalf("got %d args, want %d: %v", len(args), len(want), args)
	}
	for k, v := range want {
		p, ok := args[k]
		if !ok {
			t.Errorf("missing build arg %q", k)
			continue
		}
		if p == nil || *p != v {
			t.Errorf("build arg %q = %v, want %q", k, p, v)
		}
	}
	for _, k := range []string{"bad-name", "1NUMBER", ""} {
		if _, ok := args[k]; ok {
			t.Errorf("invalid build arg %q must be skipped", k)
		}
	}
}

func TestValidBuildArgName(t *testing.T) {
	valid := []string{"FOO", "_foo", "foo123", "A_B_C"}
	invalid := []string{"", "1foo", "foo-bar", "foo.bar", "foo bar", "ФОО"}
	for _, name := range valid {
		if !validBuildArgName(name) {
			t.Errorf("validBuildArgName(%q) = false, want true", name)
		}
	}
	for _, name := range invalid {
		if validBuildArgName(name) {
			t.Errorf("validBuildArgName(%q) = true, want false", name)
		}
	}
}
