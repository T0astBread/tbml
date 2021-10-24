package string_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	ustring "t0ast.cc/tbml/util/string"
)

func TestTrimIndentation(t *testing.T) {
	actual := ustring.TrimIndentation(`
		Hello world!
		This is what a properly indented string looks like.

			- Can contain blanks
			- Can contain nested indentation
	`)
	assert.Equal(t, `Hello world!
This is what a properly indented string looks like.

	- Can contain blanks
	- Can contain nested indentation`, actual)
}

func TestTrimIndentationPanic(t *testing.T) {
	testCases := []struct {
		desc string

		input        string
		panicMessage string
	}{
		{
			desc: "First line not empty",

			input: `First line not empty
			`,
			panicMessage: "First line in mutliline string must be empty",
		},
		{
			desc: "Not enough lines",

			input:        "",
			panicMessage: "Must have at least two lines in multiline string",
		},
		{
			desc: "Indentation smaller than first indentation",

			input: `
				This is my first line.

			This line is not properly indented.
			`,
			panicMessage: "Indentation of line 3 (3) < first line indentation of 4",
		},
		{
			desc: "Last indentation != first - 1",

			input: `
				This is my first line.
		`,
			panicMessage: "Last line indentation must be that of first line - 1, which would be 3, but was 2",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert.PanicsWithValue(t, tC.panicMessage, func() {
				ustring.TrimIndentation(tC.input)
			})
		})
	}
}
