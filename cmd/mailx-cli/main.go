// Command mailx-cli is the binary entrypoint. All real logic lives in
// internal/mailxcli - see that package's doc comment.
package main

import (
	"os"

	"github.com/UseMailx/mailx-cli/internal/mailxcli"
)

func main() {
	os.Exit(mailxcli.Run(os.Args[1:]))
}
