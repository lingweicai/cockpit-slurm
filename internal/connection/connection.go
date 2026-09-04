package connection

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os/user"
	"strconv"
	"sync"
	"syscall"
)

// State describes the lifecycle state of a connection.
type State string

const (
	StateNew     State = "new"
	StateActive  State = "active"
	StateClosing State = "closing"
	StateClosed  State = "closed"
)

var ErrPeerIdentityUnavailable = errors.New("peer identity unavailable")

// PeerIdentity is the server-derived identity for a Unix socket peer.
type PeerIdentity struct {
	UID      uint32
	GID      uint32
	Username string
}

// UserContext is the canonical Phase 1E security context for a connection.
type UserContext struct {
	UID      uint32
	GID      uint32
	Username string
}

// Session represents one logical session for a connection.
type Session struct {
	ID           string
	ConnectionID string
	UserContext  UserContext
}

// Connection represents one accepted Unix socket connection.
type Connection struct {
	ID      string
	Session *Session
	User    UserContext
	Context context.Context
	Cancel  context.CancelFunc
	State   State

	mu      sync.Mutex
	netConn net.Conn
}

// ConnectionManager stores active connections.
type ConnectionManager interface {
	Register(*Connection) error
	Unregister(connectionID string)
	Get(connectionID string) (*Connection, bool)
	Close(connectionID string) error
	CloseAll()
	Count() int
}

type manager struct {
	mu         sync.Mutex
	connections map[string]*Connection
}

func NewConnectionManager() ConnectionManager {
	return &manager{connections: make(map[string]*Connection)}
}

func (m *manager) Register(conn *Connection) error {
	if conn == nil {
		return errors.New("connection is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.connections[conn.ID]; exists {
		return fmt.Errorf("duplicate connection ID %q", conn.ID)
	}
	m.connections[conn.ID] = conn
	return nil
}

func (m *manager) Unregister(connectionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.connections, connectionID)
}

func (m *manager) Get(connectionID string) (*Connection, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn, ok := m.connections[connectionID]
	return conn, ok
}

func (m *manager) Close(connectionID string) error {
	m.mu.Lock()
	conn, ok := m.connections[connectionID]
	if ok {
		delete(m.connections, connectionID)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return conn.Close()
}

func (m *manager) CloseAll() {
	m.mu.Lock()
	conns := make([]*Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		conns = append(conns, conn)
	}
	m.connections = make(map[string]*Connection)
	m.mu.Unlock()
	for _, conn := range conns {
		_ = conn.Close()
	}
}

func (m *manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.connections)
}

func NewConnection(conn net.Conn) (*Connection, error) {
	if conn == nil {
		return nil, errors.New("connection is nil")
	}

	id, err := newID("conn")
	if err != nil {
		return nil, err
	}

	peer, err := peerIdentity(conn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	user := UserContext{
		UID:      peer.UID,
		GID:      peer.GID,
		Username: peer.Username,
	}
	if user.UID == 0 && user.GID == 0 && user.Username == "" {
		cancel()
		return nil, ErrPeerIdentityUnavailable
	}

	session := &Session{
		ID:           mustNewID("sess"),
		ConnectionID: id,
		UserContext:  user,
	}

	c := &Connection{
		ID:      id,
		Session: session,
		User:    user,
		Context: ctx,
		Cancel:  cancel,
		State:   StateNew,
		netConn: conn,
	}
	c.State = StateActive
	return c, nil
}

func (c *Connection) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if c.State == StateClosed {
		c.mu.Unlock()
		return nil
	}
	c.State = StateClosing
	if c.Cancel != nil {
		c.Cancel()
	}
	conn := c.netConn
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	c.mu.Lock()
	c.State = StateClosed
	c.mu.Unlock()
	return nil
}

func peerIdentity(conn net.Conn) (PeerIdentity, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return PeerIdentity{}, ErrPeerIdentityUnavailable
	}

	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return PeerIdentity{}, ErrPeerIdentityUnavailable
	}

	var ucred *syscall.Ucred
	var rawErr error
	if err := rawConn.Control(func(fd uintptr) {
		ucred, rawErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return PeerIdentity{}, ErrPeerIdentityUnavailable
	}
	if rawErr != nil {
		return PeerIdentity{}, ErrPeerIdentityUnavailable
	}
	if ucred == nil {
		return PeerIdentity{}, ErrPeerIdentityUnavailable
	}

	userName := ""
	if u, err := user.LookupId(strconv.Itoa(int(ucred.Uid))); err == nil {
		userName = u.Username
	}

	return PeerIdentity{
		UID:      ucred.Uid,
		GID:      ucred.Gid,
		Username: userName,
	}, nil
}

func newID(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buf)), nil
}

func mustNewID(prefix string) string {
	id, err := newID(prefix)
	if err != nil {
		panic(err)
	}
	return id
}
