// Command brokenxray passes `xray run -test` but exits right after starting,
// like a release that is incompatible with the running system.
package main

import (
	"fmt"
	"os"
	"slices"
)

func main() {
	switch {
	case len(os.Args) > 1 && os.Args[1] == "version":
		fmt.Println("Xray 99.0.0 (broken)")
	case slices.Contains(os.Args, "-test"):
		fmt.Println("Configuration OK.")
	default:
		fmt.Fprintln(os.Stderr, "broken build")
		os.Exit(1)
	}
}
