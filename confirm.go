package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// confirm asks a yes/no question on out and reads a line from in. It exists
// so every execute-tier command that does something irreversible (spec
// section 11/25: an agent or script must never turn ambiguity into a real
// side effect) goes through the same guard rather than each command
// reimplementing its own prompt - and so tests can supply a fake reader
// instead of real stdin. skip bypasses the prompt (a --yes flag) for
// scripted/non-interactive use, where the caller has already decided.
func confirm(in io.Reader, out io.Writer, prompt string, skip bool) (bool, error) {
	if skip {
		return true, nil
	}
	fmt.Fprintf(out, "%s [y/N]: ", prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
