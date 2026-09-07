// Internal test (package config, not config_test, following the
// *_internal_test.go convention already used in tournaments/websocket/moxfield):
// resolveCommit and sanitizeCommit are unexported on purpose — the package's
// contract is Config.GitCommit — but reaching them through Load() would mean
// mutating the process environment and depending on whether the test binary
// happens to carry a VCS stamp.
package config

import "testing"

const fullSHA = "0123456789abcdef0123456789abcdef01234567"

func TestResolveCommit_PrefersTheBinaryOverThePlatform(t *testing.T) {
	tests := []struct {
		name                    string
		injected, vcs, platform string
		want                    string
	}{
		{"linker injection wins", fullSHA, "aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb", fullSHA},
		{"vcs stamp beats the platform", "", fullSHA, "bbbbbbbbbbbbbbbb", fullSHA},
		{"platform is the last resort", "", "", fullSHA, fullSHA},
		{"nothing identifies the build", "", "", "", UnknownCommit},
		{"an unusable candidate falls through", "not-a-sha", "", fullSHA, fullSHA},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveCommit(tc.injected, tc.vcs, tc.platform); got != tc.want {
				t.Errorf("resolveCommit(%q, %q, %q) = %q, want %q",
					tc.injected, tc.vcs, tc.platform, got, tc.want)
			}
		})
	}
}

func TestSanitizeCommit(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"full sha", fullSHA, fullSHA},
		{"short sha", "0123456", "0123456"},
		{"trimmed and lowercased", "  0123456ABCDEF  ", "0123456abcdef"},
		{"too short", "012345", UnknownCommit},
		{"too long", fullSHA + fullSHA, UnknownCommit},
		{"not hex", "not-a-commit-sha", UnknownCommit},
		{"empty", "", UnknownCommit},
		// The value can come from an environment variable and is served on a
		// public endpoint, so anything unexpected is reported as unknown
		// instead of echoed back.
		{"arbitrary text is not echoed", "<script>alert(1)</script>", UnknownCommit},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeCommit(tc.raw); got != tc.want {
				t.Errorf("sanitizeCommit(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
