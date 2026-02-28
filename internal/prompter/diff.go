package prompter

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// DisplayChanges prints a summary of all changes to w.
func DisplayChanges(changes []Change, w io.Writer) {
	fmt.Fprintln(w, "\nChanges:")
	for _, ch := range changes {
		switch ch.Type {
		case ChangeCreated:
			label := "new"
			if !ch.IsVai {
				label = "new stub"
			}
			fmt.Fprintf(w, "  + %s (%s)\n", ch.Path, label)
		case ChangeModified:
			fmt.Fprintf(w, "  ~ %s (modified)\n", ch.Path)
		}

		// Show preview for vai files.
		if ch.IsVai && ch.Content != "" {
			lines := strings.Split(ch.Content, "\n")
			limit := 20
			if len(lines) < limit {
				limit = len(lines)
			}
			for _, line := range lines[:limit] {
				fmt.Fprintf(w, "    %s\n", line)
			}
			if len(lines) > 20 {
				fmt.Fprintf(w, "    ... (%d more lines)\n", len(lines)-20)
			}
			fmt.Fprintln(w)
		}
	}
}

// Confirm prompts the user for confirmation and reads y/n from r.
func Confirm(r io.Reader, w io.Writer) bool {
	fmt.Fprint(w, "Accept changes? [y/N] ")
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}
