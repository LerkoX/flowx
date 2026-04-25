package docker

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseInputRequest_YAML(t *testing.T) {
	content := `
type: input
prompt: "test prompt"
timeout: 30
`
	result := parseInputRequest(content)
	assert.NotNil(t, result)
	assert.Equal(t, "input", result.Type)
	assert.Equal(t, "test prompt", result.Prompt)
	assert.Equal(t, 30, result.Timeout)
}

func TestParseInputRequest_JSON(t *testing.T) {
	content := `{"type":"confirm","prompt":"Are you sure?","timeout":60}`
	result := parseInputRequest(content)
	assert.NotNil(t, result)
	assert.Equal(t, "confirm", result.Type)
	assert.Equal(t, "Are you sure?", result.Prompt)
	assert.Equal(t, 60, result.Timeout)
}

func TestParseInputRequest_Empty(t *testing.T) {
	result := parseInputRequest("")
	assert.Nil(t, result)
}

func TestParseInputRequest_WhitespaceOnly(t *testing.T) {
	result := parseInputRequest("   \n\t  ")
	assert.Nil(t, result)
}

func TestParseInputRequest_InvalidYAML(t *testing.T) {
	content := `
type: test
  invalid: yaml
  structure: [unclosed
`
	result := parseInputRequest(content)
	assert.Nil(t, result)
}

func TestParseInputRequest_InvalidJSON(t *testing.T) {
	content := `{invalid json structure`
	result := parseInputRequest(content)
	assert.Nil(t, result)
}

func TestParseInputRequest_NoType(t *testing.T) {
	content := `{"prompt": "no type field"}`
	result := parseInputRequest(content)
	assert.Nil(t, result)
}

func TestParseInputRequest_EmptyType(t *testing.T) {
	content := `{"type": "", "prompt": "empty type"}`
	result := parseInputRequest(content)
	assert.Nil(t, result)
}

func TestStdinConn_Read(t *testing.T) {
	conn := &stdinConn{}
	n, err := conn.Read(make([]byte, 100))
	assert.Equal(t, 0, n)
	assert.Equal(t, io.EOF, err)
}

func TestStdinConn_Write(t *testing.T) {
	pipeReader, pipeWriter := io.Pipe()
	conn := &stdinConn{pipeWriter: pipeWriter}

	// Write in a goroutine to avoid deadlock
	done := make(chan error, 1)
	go func() {
		n, err := conn.Write([]byte("test data"))
		if n > 0 {
			done <- nil
		} else {
			done <- err
		}
	}()

	// Read the data to unblock the write
	buf := make([]byte, 100)
	pipeReader.Read(buf)
	pipeReader.Close()
	pipeWriter.Close()

	err := <-done
	assert.NoError(t, err)
}

func TestStdinConn_Close(t *testing.T) {
	_, pipeWriter := io.Pipe()
	conn := &stdinConn{pipeWriter: pipeWriter}
	
	err := conn.Close()
	assert.NoError(t, err)
}

func TestStdinConn_LocalAddr(t *testing.T) {
	conn := &stdinConn{}
	addr := conn.LocalAddr()
	assert.Nil(t, addr)
}

func TestStdinConn_RemoteAddr(t *testing.T) {
	conn := &stdinConn{}
	addr := conn.RemoteAddr()
	assert.Nil(t, addr)
}

func TestStdinConn_SetDeadline(t *testing.T) {
	conn := &stdinConn{}
	err := conn.SetDeadline(time.Now().Add(time.Second))
	assert.NoError(t, err)
}

func TestStdinConn_SetReadDeadline(t *testing.T) {
	conn := &stdinConn{}
	err := conn.SetReadDeadline(time.Now().Add(time.Second))
	assert.NoError(t, err)
}

func TestStdinConn_SetWriteDeadline(t *testing.T) {
	conn := &stdinConn{}
	err := conn.SetWriteDeadline(time.Now().Add(time.Second))
	assert.NoError(t, err)
}

func TestCallbackWriter_Write(t *testing.T) {
	var received []byte
	writer := &callbackWriter{
		callback: func(data []byte) {
			received = append(received, data...)
		},
	}
	
	n, err := writer.Write([]byte("hello"))
	assert.Equal(t, 5, n)
	assert.NoError(t, err)
	assert.Equal(t, []byte("hello"), received)
}

func TestCallbackWriter_WriteNilCallback(t *testing.T) {
	writer := &callbackWriter{
		callback: nil,
	}
	
	n, err := writer.Write([]byte("test"))
	assert.Equal(t, 4, n)
	assert.NoError(t, err)
}