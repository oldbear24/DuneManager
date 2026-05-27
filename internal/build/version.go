package build

// Version is injected at build time via:
//
//	-ldflags "-X github.com/oldbear24/DuneManager/internal/build.Version=v1.0.0"
//
// Falls back to "dev" when building without the flag.
var Version = "dev"
