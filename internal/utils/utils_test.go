package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFormatCommitMsg(t *testing.T) {
	var msg, formatted, expected string
	msg = `Summary

Long Body
`
	formatted = FormatCommitMsg(msg)
	expected = `Summary
    
    Long Body`
	assert.Equal(t, expected, formatted)

	msg = "Summary"
	formatted = FormatCommitMsg(msg)
	expected = "Summary"
	assert.Equal(t, expected, formatted)

}
