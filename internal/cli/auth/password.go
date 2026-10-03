package auth

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// stdinReader is shared so consecutive prompts (e.g. password + confirm)
// don't lose buffered input.
var stdinReader = bufio.NewReader(os.Stdin)

// readPassword reads a password without echo when stdin is a terminal, and
// falls back to reading a plain line when input is piped (scripts, CI).
func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	if term.IsTerminal(int(syscall.Stdin)) {
		p, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		return string(p), nil
	}

	line, err := stdinReader.ReadString('\n')
	fmt.Println()
	if err != nil && line == "" {
		return "", fmt.Errorf("read password: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
