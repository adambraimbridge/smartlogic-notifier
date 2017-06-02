package kafka

import "bytes"

type FTMessage struct {
	headers map[string]string
	value   string
}

func NewFTMessage(headers map[string]string, body string) FTMessage {
	return FTMessage{
		headers: headers,
		value:   body,
	}
}

func (m *FTMessage) Build() string {
	var buffer bytes.Buffer
	buffer.WriteString("FTMSG/1.0\n")

	for k, v := range m.headers {
		buffer.WriteString(k)
		buffer.WriteString(": ")
		buffer.WriteString(v)
		buffer.WriteString("\n")
	}
	buffer.WriteString("\n")
	buffer.WriteString(m.value)

	return buffer.String()
}
