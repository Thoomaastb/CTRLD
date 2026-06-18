package main

import (
	"fmt"
	"os"
)

// version is set by the build system via -ldflags
var version = "dev"

func main() {
	fmt.Fprintf(os.Stdout, "CTRLD %s — server control panel\n", version)
	fmt.Fprintln(os.Stdout, "Not yet implemented. Development in progress.")
	os.Exit(0)
}
