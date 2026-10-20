package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	msg := NewEnvelope("MSG001", MessagePing, json.RawMessage(`{}`))

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(msg); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	dec := NewDecoder(&buf)
	got, err := dec.Decode()
	if err != nil {
		t.Fatalf("Decode() returned error: %v", err)
	}

	if got.Protocol != ProtocolName {
		t.Fatalf("Protocol = %q, want %q", got.Protocol, ProtocolName)
	}
	if got.Version != ProtocolVersion {
		t.Fatalf("Version = %q, want %q", got.Version, ProtocolVersion)
	}
	if got.MessageID != "MSG001" {
		t.Fatalf("MessageID = %q, want %q", got.MessageID, "MSG001")
	}
	if got.Type != MessagePing {
		t.Fatalf("Type = %q, want %q", got.Type, MessagePing)
	}
	if !bytes.Equal(got.Payload, json.RawMessage(`{}`)) {
		t.Fatalf("Payload = %s, want %s", got.Payload, json.RawMessage(`{}`))
	}
}

func TestDecodeMultipleFramesFromSingleStream(t *testing.T) {
	var buf bytes.Buffer

	first := NewEnvelope("MSG001", MessagePing, json.RawMessage(`{"value":1}`))
	second := NewEnvelope("MSG002", MessagePong, json.RawMessage(`{"value":2}`))

	enc := NewEncoder(&buf)
	if err := enc.Encode(first); err != nil {
		t.Fatalf("Encode(first) returned error: %v", err)
	}
	if err := enc.Encode(second); err != nil {
		t.Fatalf("Encode(second) returned error: %v", err)
	}

	dec := NewDecoder(&buf)

	firstGot, err := dec.Decode()
	if err != nil {
		t.Fatalf("Decode(first) returned error: %v", err)
	}
	if firstGot.MessageID != "MSG001" {
		t.Fatalf("first MessageID = %q, want %q", firstGot.MessageID, "MSG001")
	}
	if firstGot.Type != MessagePing {
		t.Fatalf("first Type = %q, want %q", firstGot.Type, MessagePing)
	}
	if !bytes.Equal(firstGot.Payload, json.RawMessage(`{"value":1}`)) {
		t.Fatalf("first Payload = %s, want %s", firstGot.Payload, json.RawMessage(`{"value":1}`))
	}

	secondGot, err := dec.Decode()
	if err != nil {
		t.Fatalf("Decode(second) returned error: %v", err)
	}
	if secondGot.MessageID != "MSG002" {
		t.Fatalf("second MessageID = %q, want %q", secondGot.MessageID, "MSG002")
	}
	if secondGot.Type != MessagePong {
		t.Fatalf("second Type = %q, want %q", secondGot.Type, MessagePong)
	}
	if !bytes.Equal(secondGot.Payload, json.RawMessage(`{"value":2}`)) {
		t.Fatalf("second Payload = %s, want %s", secondGot.Payload, json.RawMessage(`{"value":2}`))
	}
}

func TestDecoderRejectsZeroLengthFrame(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x00, 0x00, 0x00})

	dec := NewDecoder(&buf)
	if _, err := dec.Decode(); err == nil {
		t.Fatal("Decode() accepted a zero-length payload")
	}
}

func TestDecoderRejectsIncompletePayload(t *testing.T) {
	var buf bytes.Buffer
	// length = 16, but payload is only 8 bytes
	buf.Write([]byte{0x00, 0x00, 0x00, 0x10})
	buf.Write([]byte("12345678"))

	dec := NewDecoder(&buf)
	if _, err := dec.Decode(); err == nil {
		t.Fatal("Decode() accepted a truncated payload")
	}
}

func TestDecoderRejectsInvalidUTF8JSON(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte{0xff, 0xfe, 0xfd}
	length := uint32(len(payload))
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], length)
	buf.Write(header[:])
	buf.Write(payload)

	dec := NewDecoder(&buf)
	if _, err := dec.Decode(); err == nil {
		t.Fatal("Decode() accepted invalid UTF-8 JSON")
	}
}

func TestDecoderRejectsUnknownEnvelopeFields(t *testing.T) {
	var buf bytes.Buffer
	message := `{"protocol":"cockpit-slurm","version":"1.0","messageId":"MSG001","type":"ping","payload":{},"unexpected":true}`
	var header [FrameHeaderSize]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(message)))
	buf.Write(header[:])
	buf.WriteString(message)

	if _, err := NewDecoder(&buf).Decode(); err == nil {
		t.Fatal("Decode() accepted an unknown envelope field")
	}
}

func TestDecoderRejectsDuplicateEnvelopeFields(t *testing.T) {
	var buf bytes.Buffer
	message := `{"protocol":"cockpit-slurm","version":"1.0","messageId":"MSG001","type":"ping","payload":{},"messageId":"MSG002"}`
	var header [FrameHeaderSize]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(message)))
	buf.Write(header[:])
	buf.WriteString(message)

	if _, err := NewDecoder(&buf).Decode(); err == nil {
		t.Fatal("Decode() accepted a duplicate envelope field")
	}
}

func TestValidateRejectsUnknownMessageType(t *testing.T) {
	e := Envelope{
		Protocol:  ProtocolName,
		Version:   ProtocolVersion,
		MessageID: "MSG001",
		Type:      MessageType("not-real"),
		Payload:   json.RawMessage(`{}`),
	}

	if err := ValidateEnvelope(e); err == nil {
		t.Fatal("ValidateEnvelope() accepted an unknown message type")
	}
}

func TestValidateRejectsMissingProtocol(t *testing.T) {
	e := Envelope{
		Version:   ProtocolVersion,
		MessageID: "MSG001",
		Type:      MessagePing,
		Payload:   json.RawMessage(`{}`),
	}

	if err := ValidateEnvelope(e); err == nil {
		t.Fatal("ValidateEnvelope() accepted a missing protocol")
	}
}

func TestValidateRejectsWrongProtocol(t *testing.T) {
	e := Envelope{
		Protocol:  "wrong-protocol",
		Version:   ProtocolVersion,
		MessageID: "MSG001",
		Type:      MessagePing,
		Payload:   json.RawMessage(`{}`),
	}

	if err := ValidateEnvelope(e); err == nil {
		t.Fatal("ValidateEnvelope() accepted a wrong protocol")
	}
}

func TestValidateRejectsMissingVersion(t *testing.T) {
	e := Envelope{
		Protocol:  ProtocolName,
		MessageID: "MSG001",
		Type:      MessagePing,
		Payload:   json.RawMessage(`{}`),
	}

	if err := ValidateEnvelope(e); err == nil {
		t.Fatal("ValidateEnvelope() accepted a missing version")
	}
}

func TestValidateRejectsWrongVersion(t *testing.T) {
	e := Envelope{
		Protocol:  ProtocolName,
		Version:   "9.9",
		MessageID: "MSG001",
		Type:      MessagePing,
		Payload:   json.RawMessage(`{}`),
	}

	if err := ValidateEnvelope(e); err == nil {
		t.Fatal("ValidateEnvelope() accepted a wrong version")
	}
}

func TestValidateRejectsMissingType(t *testing.T) {
	e := Envelope{
		Protocol:  ProtocolName,
		Version:   ProtocolVersion,
		MessageID: "MSG001",
		Payload:   json.RawMessage(`{}`),
	}

	if err := ValidateEnvelope(e); err == nil {
		t.Fatal("ValidateEnvelope() accepted a missing type")
	}
}

func TestValidateRejectsEmptyMessageID(t *testing.T) {
	e := Envelope{
		Protocol: ProtocolName,
		Version:  ProtocolVersion,
		Type:     MessagePing,
		Payload:  json.RawMessage(`{}`),
	}

	if err := ValidateEnvelope(e); err == nil {
		t.Fatal("ValidateEnvelope() accepted an empty messageId")
	}
}

func TestValidateRejectsMissingPayload(t *testing.T) {
	e := Envelope{
		Protocol:  ProtocolName,
		Version:   ProtocolVersion,
		MessageID: "MSG001",
		Type:      MessagePing,
	}

	if err := ValidateEnvelope(e); err == nil {
		t.Fatal("ValidateEnvelope() accepted a missing payload")
	}
}

func TestCapabilityMessageTypesAreKnown(t *testing.T) {
	for _, messageType := range []MessageType{MessageCapabilitiesRequest, MessageCapabilitiesResponse} {
		if !messageType.IsKnown() {
			t.Errorf("message type %q is not recognized", messageType)
		}
	}
}

func TestCapabilityEnvelopesRoundTripWithV1Framing(t *testing.T) {
	request := NewEnvelope("cap-req-1", MessageCapabilitiesRequest, json.RawMessage(`{}`))
	response := NewEnvelope("cap-req-1", MessageCapabilitiesResponse, json.RawMessage(`{"capabilities":{"nodes.drain":true,"nodes.resume":false}}`))
	var framed bytes.Buffer
	encoder := NewEncoder(&framed)
	if err := encoder.Encode(request); err != nil {
		t.Fatalf("encode capabilities request: %v", err)
	}
	if err := encoder.Encode(response); err != nil {
		t.Fatalf("encode capabilities response: %v", err)
	}
	decoder := NewDecoder(&framed)
	requestGot, err := decoder.Decode()
	if err != nil {
		t.Fatalf("decode capabilities request: %v", err)
	}
	responseGot, err := decoder.Decode()
	if err != nil {
		t.Fatalf("decode capabilities response: %v", err)
	}
	if requestGot.Type != MessageCapabilitiesRequest || responseGot.Type != MessageCapabilitiesResponse || responseGot.MessageID != request.MessageID {
		t.Fatalf("round-trip messages = %#v and %#v, want correlated v1 capability messages", requestGot, responseGot)
	}
	if _, err := DecodeCapabilitiesRequestPayload(requestGot.Payload); err != nil {
		t.Fatalf("validate request payload: %v", err)
	}
	if _, err := DecodeCapabilitiesResponsePayload(responseGot.Payload); err != nil {
		t.Fatalf("validate response payload: %v", err)
	}
}

func TestDecodeCapabilitiesRequestPayload(t *testing.T) {
	if _, err := DecodeCapabilitiesRequestPayload(json.RawMessage(`{}`)); err != nil {
		t.Fatalf("decode empty capabilities request: %v", err)
	}
	if _, err := DecodeCapabilitiesRequestPayload(json.RawMessage(`{"unexpected":true}`)); err == nil {
		t.Fatal("capabilities request accepted an unknown field")
	}
	if _, err := DecodeCapabilitiesRequestPayload(json.RawMessage(`null`)); err == nil {
		t.Fatal("capabilities request accepted a non-object payload")
	}
}

func TestDecodeCapabilitiesResponsePayloadRequiresExplicitBooleans(t *testing.T) {
	payload, err := DecodeCapabilitiesResponsePayload(json.RawMessage(`{"capabilities":{"nodes.drain":true,"nodes.resume":false}}`))
	if err != nil {
		t.Fatalf("decode capabilities response: %v", err)
	}
	if !payload.Capabilities.NodesDrain || payload.Capabilities.NodesResume {
		t.Fatalf("capabilities = %#v, want drain=true and resume=false", payload.Capabilities)
	}

	invalidPayloads := []string{
		`{"capabilities":{"nodes.drain":true}}`,
		`{"capabilities":{"nodes.drain":true,"nodes.resume":false,"jobs.submit":true}}`,
		`{"capabilities":{"nodes.drain":true,"nodes.resume":null}}`,
		`{"capabilities":{"nodes.drain":true,"nodes.resume":false},"extra":true}`,
	}
	for _, input := range invalidPayloads {
		if _, err := DecodeCapabilitiesResponsePayload(json.RawMessage(input)); err == nil {
			t.Errorf("capabilities response accepted invalid payload %s", input)
		}
	}
}

func TestDecodeNodeCommandRequestPayload(t *testing.T) {
	reason := "scheduled maintenance"
	request, err := DecodeNodeCommandRequestPayload(json.RawMessage(`{"commandId":"cmd-123","resource":"node","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"DRAIN","reason":"scheduled maintenance"}}`))
	if err != nil {
		t.Fatalf("decode drain request: %v", err)
	}
	if request.CommandID != "cmd-123" || request.Target.NodeName != "c21" || request.Parameters.State != "DRAIN" || request.Parameters.Reason == nil || *request.Parameters.Reason != reason {
		t.Fatalf("decoded request = %#v", request)
	}

	resume, err := DecodeNodeCommandRequestPayload(json.RawMessage(`{"commandId":"cmd-124","resource":"node","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"RESUME"}}`))
	if err != nil || resume.Parameters.State != "RESUME" {
		t.Fatalf("decode resume request = %#v, err = %v", resume, err)
	}

	invalid := []string{
		`{"commandId":"cmd-1","resource":"node","resource":"jobs","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"RESUME"}}`,
		`{"commandId":"cmd-1","resource":"node","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"DRAIN"}}`,
		`{"commandId":"cmd-1","resource":"node","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"DRAIN","reason":null}}`,
		`{"commandId":"cmd-1","resource":"node","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"RESUME","reason":"unexpected"}}`,
		`{"commandId":"cmd-1","resource":"node","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"RESUME","reason":null}}`,
		`{"commandId":"cmd-1","resource":"node","command":"update","target":{"nodeName":"c21"},"parameters":{"state":"POWER"}}`,
		`{"commandId":"cmd-1","resource":"node","command":"update","target":{"nodeName":"c21","extra":true},"parameters":{"state":"RESUME"}}`,
	}
	for _, input := range invalid {
		if _, err := DecodeNodeCommandRequestPayload(json.RawMessage(input)); err == nil {
			t.Errorf("accepted invalid command request %s", input)
		}
	}
}

func TestDecodeCommandResponseAndStatusPayloads(t *testing.T) {
	response, err := DecodeNodeCommandResponsePayload(json.RawMessage(`{"commandId":"cmd-123","resource":"node","command":"update","status":"succeeded"}`))
	if err != nil || response.Status != CommandStatusSucceeded {
		t.Fatalf("decode command response = %#v, err = %v", response, err)
	}
	status, err := DecodeCommandStatusPayload(json.RawMessage(`{"commandId":"cmd-123","status":"unknown"}`))
	if err != nil || status.Status != CommandStatusUnknown {
		t.Fatalf("decode command status = %#v, err = %v", status, err)
	}
	if _, err := DecodeCommandStatusPayload(json.RawMessage(`{"commandId":"cmd-123","status":"accepted"}`)); err == nil {
		t.Fatal("accepted an internal command lifecycle state")
	}
}

type partialWriter struct {
	chunkSize int
	buf       bytes.Buffer
}

func (w *partialWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.chunkSize <= 0 {
		w.chunkSize = len(p)
	}
	limit := min(len(p), w.chunkSize)
	written, err := w.buf.Write(p[:limit])
	if err != nil {
		return written, err
	}
	return written, nil
}

func (w *partialWriter) Bytes() []byte {
	return w.buf.Bytes()
}

func TestEncoderWritesFullFrame(t *testing.T) {
	msg := NewEnvelope("MSG001", MessageHello, json.RawMessage(`{"client":"test"}`))
	var buf bytes.Buffer

	enc := NewEncoder(&buf)
	if err := enc.Encode(msg); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	if len(buf.Bytes()) < 4 {
		t.Fatal("encoded frame too short")
	}

	length := binary.BigEndian.Uint32(buf.Bytes()[:4])
	if int(length) != len(buf.Bytes())-4 {
		t.Fatalf("length prefix = %d, actual payload length = %d", length, len(buf.Bytes())-4)
	}

	if !strings.Contains(buf.String(), "\"type\":\"hello\"") {
		t.Fatal("encoded payload did not contain hello type")
	}
}

func TestEncoderHandlesPartialWriter(t *testing.T) {
	msg := NewEnvelope("MSG001", MessagePing, json.RawMessage(`{"ok":true}`))
	pw := &partialWriter{chunkSize: 7}

	enc := NewEncoder(pw)
	if err := enc.Encode(msg); err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}

	data := pw.Bytes()
	if len(data) < 4 {
		t.Fatal("encoded frame too short")
	}

	length := binary.BigEndian.Uint32(data[:4])
	if int(length) != len(data)-4 {
		t.Fatalf("length prefix = %d, actual payload length = %d", length, len(data)-4)
	}

	if !bytes.Contains(data, []byte("\"type\":\"ping\"")) {
		t.Fatal("encoded payload did not contain ping type")
	}
}

func maxEnvelopePayloadSizeForLimit(limit int) int {
	lo, hi := 0, limit
	best := 0

	for lo <= hi {
		mid := lo + (hi-lo)/2
		payload := fmt.Sprintf(`{"data":"%s"}`, strings.Repeat("a", mid))
		msg := NewEnvelope("MSG001", MessagePing, json.RawMessage(payload))

		encoded, err := json.Marshal(msg)
		if err != nil {
			panic(err)
		}

		if len(encoded) <= limit {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}

	return best
}

func TestMaxPayloadSizeBoundary(t *testing.T) {
	payloadLen := maxEnvelopePayloadSizeForLimit(MaxPayloadSize)
	payload := fmt.Sprintf(`{"data":"%s"}`, strings.Repeat("a", payloadLen))
	msg := NewEnvelope("MSG001", MessagePing, json.RawMessage(payload))

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(msg); err != nil {
		t.Fatalf("Encode() with a valid message at the size limit returned error: %v", err)
	}

	encoded, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal(msg) returned error: %v", err)
	}

	length := binary.BigEndian.Uint32(buf.Bytes()[:4])
	if int(length) != len(buf.Bytes())-4 {
		t.Fatalf("length prefix = %d, actual payload length = %d", length, len(buf.Bytes())-4)
	}
	if int(length) != len(encoded) {
		t.Fatalf("length prefix = %d, want encoded-message length = %d", length, len(encoded))
	}
}

func TestMaxPayloadSizeRejectsOversizedPayload(t *testing.T) {
	payloadLen := maxEnvelopePayloadSizeForLimit(MaxPayloadSize) + 1
	payload := fmt.Sprintf(`{"data":"%s"}`, strings.Repeat("a", payloadLen))
	msg := NewEnvelope("MSG001", MessagePing, json.RawMessage(payload))

	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(msg); err == nil {
		t.Fatal("Encode() accepted a valid message larger than MaxPayloadSize")
	}

	if buf.Len() != 0 {
		t.Fatalf("Encode() wrote %d bytes for oversized message, want 0", buf.Len())
	}
}
