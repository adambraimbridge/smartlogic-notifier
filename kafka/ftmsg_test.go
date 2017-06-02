package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const testFTMessageResponse = "FTMSG/1.0\ntest: test2\n\nTest Message"

func Test_FTMessage_Build(t *testing.T) {
	ft := NewFTMessage(map[string]string{"test": "test2"}, "Test Message")

	ftm := ft.Build()
	assert.EqualValues(t, testFTMessageResponse, ftm)
}
