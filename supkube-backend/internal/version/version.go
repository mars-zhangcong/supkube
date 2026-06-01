// Package version exposes the build-time version + stamp so /status can
// return them. Set via -ldflags at build time:
//
//	go build -ldflags "-X github.com/supkube/supkube-backend/internal/version.Version=0.8.5-alpha-step6 \
//	                   -X github.com/supkube/supkube-backend/internal/version.BuildStamp=260522-2310" .
//
// The Dockerfile does this automatically using a CACHEBUST arg → timestamp.
// Falling back to "dev" / "local" when ldflags weren't passed keeps `go run`
// in development working without surprises.
package version

// Version is the SupKube release identifier. Overridden via ldflags.
var Version = "dev"

// BuildStamp is YYMMDD-HHMM of the build moment (UTC). Overridden via ldflags.
// Surface this in /status + the UI header so users can spot "my browser
// cached the old bundle" vs "the cluster is running the old image".
var BuildStamp = "local"
