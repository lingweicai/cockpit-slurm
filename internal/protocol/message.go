package protocol

// MessageType identifies an IPC message type.
type MessageType string

const (
	MessageHello                MessageType = "hello"
	MessageHelloResponse        MessageType = "hello-response"
	MessageQuery                MessageType = "query"
	MessageQueryResponse        MessageType = "query-response"
	MessageSubscribe            MessageType = "subscribe"
	MessageSubscribeResponse    MessageType = "subscribe-response"
	MessageUnsubscribe          MessageType = "unsubscribe"
	MessageUnsubscribeResponse  MessageType = "unsubscribe-response"
	MessageCommand              MessageType = "command"
	MessageCommandResponse      MessageType = "command-response"
	MessageCommandStatus        MessageType = "command-status"
	MessageCapabilitiesRequest  MessageType = "capabilities-request"
	MessageCapabilitiesResponse MessageType = "capabilities-response"
	MessageEvent                MessageType = "event"
	MessageError                MessageType = "error"
	MessagePing                 MessageType = "ping"
	MessagePong                 MessageType = "pong"
	MessageClose                MessageType = "close"
)

func (t MessageType) IsKnown() bool {
	switch t {
	case MessageHello,
		MessageHelloResponse,
		MessageQuery,
		MessageQueryResponse,
		MessageSubscribe,
		MessageSubscribeResponse,
		MessageUnsubscribe,
		MessageUnsubscribeResponse,
		MessageCommand,
		MessageCommandResponse,
		MessageCommandStatus,
		MessageCapabilitiesRequest,
		MessageCapabilitiesResponse,
		MessageEvent,
		MessageError,
		MessagePing,
		MessagePong,
		MessageClose:
		return true
	default:
		return false
	}
}
