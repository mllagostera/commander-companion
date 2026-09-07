package config

import (
	"os"
	"runtime/debug"
	"strings"
)

// buildCommit is injected at link time:
//
//	go build -ldflags "-X github.com/usuario/commander-companion-backend/internal/config.buildCommit=$SHA"
//
// which is what backend/Dockerfile's GIT_COMMIT build argument feeds. It stays
// empty on a plain `go build`/`go run`, hence the fallbacks in gitCommit.
//
// It has to be a package-level var: the linker's -X can only write into one.
//
//nolint:gochecknoglobals // written by the linker, not by the program
var buildCommit string

// UnknownCommit is what GET /health reports when nothing identifies the build:
// no linker injection, no VCS stamp in the binary, no platform variable. It is
// a value, not an error — the endpoint still has to answer.
const UnknownCommit = "unknown"

// renderCommitEnv is set by Render on every service deployed from a Git repo.
// It is the fallback that makes the marker work on the current deployment
// without changing how Render builds the image (see ADR-0020): Render builds
// backend/Dockerfile with backend/ as its context, so the binary carries no
// VCS stamp — the repository's .git lives one directory up, outside it.
const renderCommitEnv = "RENDER_GIT_COMMIT"

// Bounds of what sanitizeCommit accepts as a commit: from 7 (git's short SHA)
// to 64 (a full SHA-256 object ID, should the repository ever be converted).
const (
	minCommitLen = 7
	maxCommitLen = 64
)

// gitCommit resolves which source revision this binary was built from, from
// the three places that can know it in this deployment.
func gitCommit() string {
	return resolveCommit(buildCommit, vcsRevision(), os.Getenv(renderCommitEnv))
}

// resolveCommit picks the first usable candidate in descending order of
// authority: what the linker stamped into this specific binary, what the Go
// toolchain recorded from the VCS at build time, and only then what the
// hosting platform says it deployed — the platform describes the deploy, the
// first two describe the binary actually running.
func resolveCommit(injected, vcs, platform string) string {
	for _, candidate := range []string{injected, vcs, platform} {
		if c := sanitizeCommit(candidate); c != UnknownCommit {
			return c
		}
	}
	return UnknownCommit
}

// vcsRevision reads the vcs.revision stamp the Go toolchain embeds when it
// builds from a git checkout (-buildvcs=auto). Absent in a container build
// whose context excludes .git, and absent in tests.
func vcsRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			return setting.Value
		}
	}
	return ""
}

// sanitizeCommit keeps the public /health body bounded and predictable: the
// value can come from an environment variable, so anything that is not a
// plausible git object ID is reported as unknown rather than echoed back.
func sanitizeCommit(raw string) string {
	c := strings.TrimSpace(raw)
	if len(c) < minCommitLen || len(c) > maxCommitLen {
		return UnknownCommit
	}
	c = strings.ToLower(c)
	for _, r := range c {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !isHex {
			return UnknownCommit
		}
	}
	return c
}
