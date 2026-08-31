package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const (
	FrameHeaderSize = 4
	MaxPayloadSize  = 4 * 1024 * 1024
)

// Encoder writes length-prefixed JSON messages.
type Encoder struct {
	w io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

func (e *Encoder) Encode(message Envelope) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}

	if len(payload) == 0 {
		return fmt.Errorf("%w: empty envelope payload", ErrInvalidMessage)
	}
	if len(payload) > MaxPayloadSize {
		return fmt.Errorf("message size %d exceeds maximum %d", len(payload), MaxPayloadSize)
	}

	var header [FrameHeaderSize]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))

	if err := writeFull(e.w, header[:]); err != nil {
		return fmt.Errorf("write message length: %w", err)
	}
	if err := writeFull(e.w, payload); err != nil {
		return fmt.Errorf("write message payload: %w", err)
	}

	return nil
}

func writeFull(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}

// Decoder reads one length-prefixed message from a stream.
type Decoder struct {
	r io.Reader
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

func (d *Decoder) Decode() (Envelope, error) {
	var envelope Envelope
	var header [FrameHeaderSize]byte

	if _, err := io.ReadFull(d.r, header[:]); err != nil {
		return envelope, fmt.Errorf("read message length: %w", err)
	}

	length := binary.BigEndian.Uint32(header[:])
	if length == 0 {
		return envelope, fmt.Errorf("%w: zero-length frame", ErrInvalidMessage)
	}
	if length > MaxPayloadSize {
		return envelope, fmt.Errorf("message length %d exceeds maximum %d", length, MaxPayloadSize)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(d.r, payload); err != nil {
		return envelope, fmt.Errorf("read message payload: %w", err)
	}

	if err := json.Unmarshal(payload, &envelope); err != nil {
		return envelope, fmt.Errorf("decode JSON: %w", err)
	}

	if err := ValidateEnvelope(envelope); err != nil {
		return envelope, err
	}

	return envelope, nil
}
