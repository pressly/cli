package textutil

import "strings"

// Wrap wraps text to width at word boundaries.
func Wrap(text string, width int) []string {
	words := strings.Fields(text)
	var (
		lines         []string
		currentLine   []string
		currentLength int
	)
	for _, word := range words {
		if currentLength+len(word)+1 > width {
			if len(currentLine) > 0 {
				lines = append(lines, strings.Join(currentLine, " "))
				currentLine = []string{word}
				currentLength = len(word)
			} else {
				lines = append(lines, word)
			}
		} else {
			currentLine = append(currentLine, word)
			if currentLength == 0 {
				currentLength = len(word)
			} else {
				currentLength += len(word) + 1
			}
		}
	}
	if len(currentLine) > 0 {
		lines = append(lines, strings.Join(currentLine, " "))
	}
	return lines
}
