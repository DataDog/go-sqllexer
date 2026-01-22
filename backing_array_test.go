package sqllexer

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

// BackingArrayTestResult holds the output from a SQL processing function
// for backing array verification tests.
type BackingArrayTestResult struct {
	SQL      string
	Metadata *StatementMetadata
}

// BackingArrayTestFunc is the signature for SQL processing functions under test.
type BackingArrayTestFunc func(input string) (BackingArrayTestResult, error)

// stringSharesMemory checks if str's backing array overlaps with source's memory range.
// This uses unsafe to inspect the string header pointers.
func stringSharesMemory(str, source string) bool {
	if len(str) == 0 || len(source) == 0 {
		return false
	}
	strPtr := uintptr((*[2]uintptr)(unsafe.Pointer(&str))[0])
	srcPtr := uintptr((*[2]uintptr)(unsafe.Pointer(&source))[0])
	srcEnd := srcPtr + uintptr(len(source))
	return strPtr >= srcPtr && strPtr < srcEnd
}

// BackingArrayTestCase defines a test case for backing array verification.
type BackingArrayTestCase struct {
	Name         string
	Input        string
	MinInputSize int // Minimum expected input size for sanity check
}

// BackingArrayTestCases returns test cases for verifying that SQL processing
// functions don't return strings that pin large backing arrays.
// These cases use literal values so they work with all three APIs.
func BackingArrayTestCases() []BackingArrayTestCase {
	return []BackingArrayTestCase{
		{
			Name:         "large_comment_padding",
			Input:        "/*" + strings.Repeat(" ", 100*1024) + "*/ SELECT * FROM users WHERE id = 1",
			MinInputSize: 100 * 1024,
		},
		{
			Name:         "large_whitespace_padding",
			Input:        strings.Repeat(" ", 50*1024) + "SELECT id FROM orders WHERE x = 1" + strings.Repeat(" ", 50*1024),
			MinInputSize: 100 * 1024,
		},
		{
			Name:         "large_string_literal",
			Input:        "SELECT * FROM users WHERE bio = '" + strings.Repeat("x", 100*1024) + "'",
			MinInputSize: 100 * 1024,
		},
		{
			Name:         "huge_input",
			Input:        "/*" + strings.Repeat(".", 1024*1024) + "*/ SELECT 1",
			MinInputSize: 1024 * 1024,
		},
	}
}

// RunBackingArrayTests runs the backing array verification tests using the provided
// processing function. This is the main entry point for the shared test harness.
func RunBackingArrayTests(t *testing.T, name string, processFunc BackingArrayTestFunc) {
	testCases := BackingArrayTestCases()

	t.Run(name, func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				// Sanity check on input size
				assert.True(t, len(tc.Input) >= tc.MinInputSize,
					"input should be at least %d bytes, got %d", tc.MinInputSize, len(tc.Input))

				result, err := processFunc(tc.Input)
				assert.NoError(t, err)

				// Verify the output SQL doesn't share memory with input
				assert.False(t, stringSharesMemory(result.SQL, tc.Input),
					"output SQL should not share memory with input")

				// Verify metadata strings don't share memory with input (if metadata is returned)
				if result.Metadata != nil {
					for _, table := range result.Metadata.Tables {
						assert.False(t, stringSharesMemory(table, tc.Input),
							"table name %q should not share memory with input", table)
					}
					for _, comment := range result.Metadata.Comments {
						assert.False(t, stringSharesMemory(comment, tc.Input),
							"comment should not share memory with input")
					}
					for _, command := range result.Metadata.Commands {
						assert.False(t, stringSharesMemory(command, tc.Input),
							"command %q should not share memory with input", command)
					}
					for _, proc := range result.Metadata.Procedures {
						assert.False(t, stringSharesMemory(proc, tc.Input),
							"procedure %q should not share memory with input", proc)
					}
				}
			})
		}
	})
}
