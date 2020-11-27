package version

import (
	"fmt"
	"os"
	"runtime"
)

var (
	Version = "v0.1.0"
	GitSHA  = "Not provided."
)

// PrintVersionAndExit prints versions from the array returned by Info() and exit
func PrintVersionAndExit(apiVersion string) {
	for _, i := range Info(apiVersion) {
		fmt.Printf("%v\n", i)
	}
	os.Exit(0)
}

// Info returns an array of various service versions
func Info(apiVersion string) []string {
	return []string{
		fmt.Sprintf("API Version: %s", apiVersion),
		fmt.Sprintf("Version: %s", Version),
		fmt.Sprintf("Git SHA: %s", GitSHA),
		fmt.Sprintf("Go Version: %s", runtime.Version()),
		fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
