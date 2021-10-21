package string

import (
	"fmt"
	"strings"
)

func TrimIndentation(str string) string {
	lines := strings.Split(str, "\n")

	if len(lines) < 2 {
		panic("Must have at least two lines in multiline string")
	}
	if len(lines[0]) > 0 {
		panic("First line in mutliline string must be empty")
	}

	lastLine := lines[len(lines)-1]
	lines = lines[1 : len(lines)-1]
	firstIndent := getFirstIndentation(lines)
	if expected, actual := firstIndent-1, getLineIndentation(lastLine); expected != actual {
		panic(fmt.Sprintf("Last line indentation must be that of first line - 1, which would be %d, but was %d", expected, actual))
	}

	outLines := make([]string, len(lines))
	for i, line := range lines {
		if len(line) == 0 {
			outLines[i] = line
		} else {
			lineIndent := getLineIndentation(line)
			if lineIndent < firstIndent {
				panic(fmt.Sprintf("Indentation of line %d (%d) < first line indentation of %d", i+1, lineIndent, firstIndent))
			}
			outLines[i] = line[int(firstIndent):]
		}
	}
	return strings.Join(outLines, "\n")
}

func getFirstIndentation(lines []string) uint {
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if len(line) > 0 {
			return getLineIndentation(line)
		}
	}
	return 0
}

func getLineIndentation(line string) uint {
	var i int
	for ; i < len(line) && line[i] == '\t'; i++ {
	}
	return uint(i)
}
