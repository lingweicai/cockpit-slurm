package ledger

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	DefaultPath   = "/var/lib/cockpit-slurm/commands.db"
	SchemaVersion = 3
	Retention     = 30 * 24 * time.Hour
)

var (
	ErrUnsupportedSchema = errors.New("unsupported command ledger schema")
	ErrIntegrityCheck    = errors.New("command ledger integrity check failed")
	ErrPersistenceFailed = errors.New("command ledger persistence failed")
	ErrAdmissionBlocked  = errors.New("command admission is blocked")
	ErrCommandIDConflict = errors.New("command ID conflicts with an existing request")
	ErrInvalidTransition = errors.New("invalid command ledger transition")
	ErrCommandNotFound   = errors.New("command ledger record not found")
)

type Status string

const (
	StatusAuthorizationPending   Status = "authorization_pending"
	StatusAuthorizationUncertain Status = "authorization_uncertain"
	StatusAuthorizationFailed    Status = "authorization_failed"
	StatusAccepted               Status = "accepted"
	StatusRunning                Status = "running"
	StatusSucceeded              Status = "succeeded"
	StatusFailed                 Status = "failed"
	StatusDenied                 Status = "denied"
	StatusUnknown                Status = "unknown"
)

type Claim struct {
	CallerUID          uint32
	CommandID          string
	RequestFingerprint []byte
	RequestPayload     []byte
	RequestingUsername string
	RequestingGID      uint32
	RequestMessageID   string
	ServiceUID         uint32
	ServiceUsername    string
	Resource           string
	Command            string
	OperationID        string
	RequiredCapability string
	TargetNode         string
}

type Record struct {
	CallerUID             uint32
	CommandID             string
	RequestFingerprint    []byte
	RequestPayload        []byte
	RequestingUsername    string
	RequestingGID         uint32
	RequestMessageID      string
	ServiceUID            uint32
	ServiceUsername       string
	Resource              string
	Command               string
	OperationID           string
	RequiredCapability    string
	TargetNode            string
	Status                Status
	AdminLevel            string
	AuthorizationMetadata []byte
	ExecutionUID          *uint32
	ExecutionUsername     string
	ExecutionBoundary     string
	ResultPayload         []byte
	ErrorPayload          []byte
	CreatedAt             time.Time
	UpdatedAt             time.Time
	ExpiresAt             time.Time
}

type TransitionData struct {
	AdminLevel            string
	AuthorizationMetadata []byte
	ExecutionUID          *uint32
	ExecutionUsername     string
	ExecutionBoundary     string
	ResultPayload         []byte
	ErrorPayload          []byte
}

type Ledger struct {
	db                *sql.DB
	path              string
	writeMu           sync.Mutex
	gateMu            sync.RWMutex
	blocked           bool
	commitTransaction func(*sql.Tx) error
}

func Open(path string) (*Ledger, error) {
	if path == "" {
		path = DefaultPath
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve command ledger path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0750); err != nil {
		return nil, fmt.Errorf("create command ledger directory: %w", err)
	}
	file, err := os.OpenFile(absolutePath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("create command ledger database: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close command ledger database file: %w", err)
	}
	if err := os.Chmod(absolutePath, 0600); err != nil {
		return nil, fmt.Errorf("restrict command ledger database permissions: %w", err)
	}

	db, err := sql.Open("sqlite", absolutePath)
	if err != nil {
		return nil, fmt.Errorf("open command ledger database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ledger := &Ledger{db: db, path: absolutePath}
	if err := ledger.configure(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ledger.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ledger.integrityCheck(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ledger.recoverInterrupted(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ledger.purgeExpired(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return ledger, nil
}

func (l *Ledger) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}

func (l *Ledger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *Ledger) AdmissionBlocked() bool {
	if l == nil {
		return true
	}
	l.gateMu.RLock()
	defer l.gateMu.RUnlock()
	return l.blocked
}

func (l *Ledger) ReserveAuthorization(ctx context.Context, claim Claim) (Record, bool, error) {
	if err := validateClaim(claim); err != nil {
		return Record{}, false, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	if l.isBlocked() {
		return Record{}, false, ErrAdmissionBlocked
	}

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return Record{}, false, l.blockPersistence("begin authorization claim", err)
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `DELETE FROM commands WHERE expires_at <= ?`, now.Unix()); err != nil {
		_ = tx.Rollback()
		return Record{}, false, l.blockPersistence("expire command records", err)
	}
	result, err := tx.ExecContext(ctx, `
INSERT INTO commands (
	caller_uid, command_id, request_fingerprint, request_payload,
	requesting_username, requesting_gid, request_message_id,
	service_uid, service_username,
	resource, command, operation_id, required_capability,
	target_node, status, created_at, updated_at, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (caller_uid, command_id) DO NOTHING
`, claim.CallerUID, claim.CommandID, claim.RequestFingerprint, claim.RequestPayload,
		claim.RequestingUsername, claim.RequestingGID, claim.RequestMessageID,
		claim.ServiceUID, claim.ServiceUsername,
		claim.Resource, claim.Command, claim.OperationID,
		claim.RequiredCapability, claim.TargetNode, StatusAuthorizationPending,
		now.Unix(), now.Unix(), now.Add(Retention).Unix())
	if err != nil {
		_ = tx.Rollback()
		return Record{}, false, l.blockPersistence("write authorization claim", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return Record{}, false, l.blockPersistence("inspect authorization claim", err)
	}
	created := rows == 1
	record, err := selectRecord(ctx, tx, claim.CallerUID, claim.CommandID)
	if err != nil {
		_ = tx.Rollback()
		return Record{}, false, l.blockPersistence("read authorization claim", err)
	}
	if !bytes.Equal(record.RequestFingerprint, claim.RequestFingerprint) {
		_ = tx.Rollback()
		return Record{}, false, ErrCommandIDConflict
	}
	if err := l.commit(tx); err != nil {
		_ = tx.Rollback()
		return Record{}, false, l.blockPersistence("commit authorization claim", err)
	}
	return record, created, nil
}

func (l *Ledger) Lookup(ctx context.Context, callerUID uint32, commandID string) (Record, bool, error) {
	if l == nil || l.db == nil {
		return Record{}, false, ErrPersistenceFailed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var record Record
	err := scanRecord(l.db.QueryRowContext(ctx, `SELECT `+recordColumns+` FROM commands WHERE caller_uid = ? AND command_id = ? AND expires_at > ?`, callerUID, commandID, time.Now().UTC().Unix()), &record)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, false, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Record{}, false, fmt.Errorf("read command ledger record: %w", err)
	}
	if err != nil {
		return Record{}, false, l.blockPersistence("read command ledger record", err)
	}
	return record, true, nil
}

func (l *Ledger) purgeExpired(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := l.db.ExecContext(ctx, `DELETE FROM commands WHERE expires_at <= ?`, time.Now().UTC().Unix()); err != nil {
		return fmt.Errorf("purge expired command ledger records: %w", err)
	}
	return nil
}

func (l *Ledger) Transition(ctx context.Context, callerUID uint32, commandID string, expected, next Status, data TransitionData) error {
	if !allowedTransition(expected, next) {
		return fmt.Errorf("%w: %s to %s", ErrInvalidTransition, expected, next)
	}
	if next == StatusAccepted && (data.AdminLevel != "admin" || len(data.AuthorizationMetadata) == 0) {
		return fmt.Errorf("%w: accepted status requires current admin authorization metadata", ErrInvalidTransition)
	}
	if next == StatusRunning && (data.ExecutionUID == nil || data.ExecutionUsername == "" || data.ExecutionBoundary == "") {
		return fmt.Errorf("%w: running status requires execution identity and boundary", ErrInvalidTransition)
	}
	if (next == StatusDenied || next == StatusAuthorizationFailed) && len(data.ErrorPayload) == 0 {
		return fmt.Errorf("%w: terminal authorization status requires an error payload", ErrInvalidTransition)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	if l.isBlocked() {
		return ErrAdmissionBlocked
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return l.blockPersistence("begin command transition", err)
	}
	result, err := tx.ExecContext(ctx, `
UPDATE commands SET
	status = ?, updated_at = ?,
	admin_level = COALESCE(NULLIF(?, ''), admin_level),
	authorization_metadata = COALESCE(?, authorization_metadata),
	execution_uid = COALESCE(?, execution_uid),
	execution_username = COALESCE(NULLIF(?, ''), execution_username),
	execution_boundary = COALESCE(NULLIF(?, ''), execution_boundary),
	result_payload = COALESCE(?, result_payload),
	error_payload = COALESCE(?, error_payload)
WHERE caller_uid = ? AND command_id = ? AND status = ?
`, next, time.Now().UTC().Unix(), data.AdminLevel, nullableBytes(data.AuthorizationMetadata),
		nullableUID(data.ExecutionUID), data.ExecutionUsername, data.ExecutionBoundary,
		nullableBytes(data.ResultPayload), nullableBytes(data.ErrorPayload), callerUID,
		commandID, expected)
	if err != nil {
		_ = tx.Rollback()
		return l.blockPersistence("write command transition", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return l.blockPersistence("inspect command transition", err)
	}
	if rows != 1 {
		_ = tx.Rollback()
		return l.transitionConflict(ctx, callerUID, commandID)
	}
	if err := l.commit(tx); err != nil {
		_ = tx.Rollback()
		return l.blockPersistence("commit command transition", err)
	}
	return nil
}

func (l *Ledger) isBlocked() bool {
	l.gateMu.RLock()
	defer l.gateMu.RUnlock()
	return l.blocked
}

func (l *Ledger) blockPersistence(operation string, err error) error {
	l.gateMu.Lock()
	l.blocked = true
	l.gateMu.Unlock()
	return fmt.Errorf("%w: %s: %v", ErrPersistenceFailed, operation, err)
}

func (l *Ledger) transitionConflict(ctx context.Context, callerUID uint32, commandID string) error {
	_, found, err := l.Lookup(ctx, callerUID, commandID)
	if err != nil {
		return l.blockPersistence("reconcile command transition", err)
	}
	if !found {
		return ErrCommandNotFound
	}
	return ErrInvalidTransition
}

func validateClaim(claim Claim) error {
	if claim.CommandID == "" || claim.RequestingUsername == "" || claim.RequestMessageID == "" || claim.ServiceUsername == "" || len(claim.RequestFingerprint) == 0 || len(claim.RequestPayload) == 0 || claim.TargetNode == "" {
		return fmt.Errorf("%w: incomplete authorization claim", ErrInvalidTransition)
	}
	if claim.Resource != "node" || claim.Command != "update" {
		return fmt.Errorf("%w: unsupported resource/command", ErrInvalidTransition)
	}
	if (claim.OperationID != "node.drain" || claim.RequiredCapability != "nodes.drain") &&
		(claim.OperationID != "node.resume" || claim.RequiredCapability != "nodes.resume") {
		return fmt.Errorf("%w: operation/capability mismatch", ErrInvalidTransition)
	}
	return nil
}

func allowedTransition(from, to Status) bool {
	switch from {
	case StatusAuthorizationPending:
		return to == StatusAccepted || to == StatusDenied || to == StatusAuthorizationFailed || to == StatusAuthorizationUncertain
	case StatusAccepted:
		return to == StatusRunning || to == StatusAuthorizationUncertain
	case StatusRunning:
		return to == StatusSucceeded || to == StatusFailed || to == StatusUnknown
	default:
		return false
	}
}

func nullableBytes(value []byte) any {
	if value == nil {
		return nil
	}
	return value
}

func nullableUID(value *uint32) any {
	if value == nil {
		return nil
	}
	return *value
}

const recordColumns = `caller_uid, command_id, request_fingerprint, request_payload,
requesting_username, requesting_gid, request_message_id, service_uid, service_username,
resource, command, operation_id, required_capability,
target_node, status, admin_level, authorization_metadata, execution_uid,
execution_username, execution_boundary, result_payload, error_payload,
created_at, updated_at, expires_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func selectRecord(ctx context.Context, tx *sql.Tx, callerUID uint32, commandID string) (Record, error) {
	var record Record
	err := scanRecord(tx.QueryRowContext(ctx, `SELECT `+recordColumns+` FROM commands WHERE caller_uid = ? AND command_id = ?`, callerUID, commandID), &record)
	return record, err
}

func scanRecord(row rowScanner, record *Record) error {
	var status string
	var adminLevel, executionUsername, executionBoundary sql.NullString
	var authorizationMetadata, resultPayload, errorPayload []byte
	var executionUID sql.NullInt64
	var createdAt, updatedAt, expiresAt int64
	if err := row.Scan(
		&record.CallerUID, &record.CommandID, &record.RequestFingerprint, &record.RequestPayload,
		&record.RequestingUsername, &record.RequestingGID, &record.RequestMessageID,
		&record.ServiceUID, &record.ServiceUsername,
		&record.Resource, &record.Command, &record.OperationID,
		&record.RequiredCapability, &record.TargetNode, &status, &adminLevel,
		&authorizationMetadata, &executionUID, &executionUsername, &executionBoundary,
		&resultPayload, &errorPayload, &createdAt, &updatedAt, &expiresAt,
	); err != nil {
		return err
	}
	record.Status = Status(status)
	record.AdminLevel = adminLevel.String
	record.AuthorizationMetadata = authorizationMetadata
	if executionUID.Valid {
		uid := uint32(executionUID.Int64)
		record.ExecutionUID = &uid
	}
	record.ExecutionUsername = executionUsername.String
	record.ExecutionBoundary = executionBoundary.String
	record.ResultPayload = resultPayload
	record.ErrorPayload = errorPayload
	record.CreatedAt = time.Unix(createdAt, 0).UTC()
	record.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	record.ExpiresAt = time.Unix(expiresAt, 0).UTC()
	return nil
}

func (l *Ledger) configure() error {
	if _, err := l.db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		return fmt.Errorf("configure command ledger busy timeout: %w", err)
	}
	var journalMode string
	if err := l.db.QueryRow(`PRAGMA journal_mode = WAL`).Scan(&journalMode); err != nil {
		return fmt.Errorf("configure command ledger WAL mode: %w", err)
	}
	if journalMode != "wal" {
		return fmt.Errorf("configure command ledger WAL mode: got %q", journalMode)
	}
	if _, err := l.db.Exec(`PRAGMA synchronous = FULL`); err != nil {
		return fmt.Errorf("configure command ledger synchronous mode: %w", err)
	}
	if _, err := l.db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("configure command ledger foreign keys: %w", err)
	}
	if _, err := l.db.Exec(`PRAGMA temp_store = MEMORY`); err != nil {
		return fmt.Errorf("configure command ledger temporary storage: %w", err)
	}
	return nil
}

func (l *Ledger) migrate() error {
	var version int
	if err := l.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read command ledger schema version: %w", err)
	}
	if version > SchemaVersion {
		return fmt.Errorf("%w: database version %d, supported version %d", ErrUnsupportedSchema, version, SchemaVersion)
	}
	for version < SchemaVersion {
		nextVersion := version + 1
		tx, err := l.db.Begin()
		if err != nil {
			return fmt.Errorf("begin command ledger migration %d: %w", nextVersion, err)
		}
		if err := applyMigration(tx, nextVersion); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply command ledger migration %d: %w", nextVersion, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, nextVersion)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("set command ledger schema version %d: %w", nextVersion, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit command ledger migration %d: %w", nextVersion, err)
		}
		version = nextVersion
	}
	return nil
}

func applyMigration(tx *sql.Tx, version int) error {
	switch version {
	case 1:
		_, err := tx.Exec(`
CREATE TABLE commands (
	caller_uid INTEGER NOT NULL CHECK (caller_uid >= 0),
	command_id TEXT NOT NULL CHECK (length(command_id) > 0),
	request_fingerprint BLOB NOT NULL,
	request_payload BLOB NOT NULL,
	requesting_username TEXT NOT NULL CHECK (length(requesting_username) > 0),
	resource TEXT NOT NULL CHECK (resource = 'node'),
	command TEXT NOT NULL CHECK (command = 'update'),
	operation_id TEXT NOT NULL CHECK (operation_id IN ('node.drain', 'node.resume')),
	required_capability TEXT NOT NULL CHECK (required_capability IN ('nodes.drain', 'nodes.resume')),
	target_node TEXT NOT NULL CHECK (length(target_node) > 0),
	status TEXT NOT NULL CHECK (status IN (
		'authorization_pending', 'authorization_uncertain', 'authorization_failed',
		'accepted', 'running', 'succeeded', 'failed', 'denied', 'unknown'
	)),
	admin_level TEXT,
	authorization_metadata BLOB,
	execution_uid INTEGER,
	execution_username TEXT,
	execution_boundary TEXT,
	result_payload BLOB,
	error_payload BLOB,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	PRIMARY KEY (caller_uid, command_id)
);
CREATE INDEX commands_expires_at_idx ON commands(expires_at);
		`)
		return err
	case 2:
		_, err := tx.Exec(`
ALTER TABLE commands ADD COLUMN requesting_gid INTEGER NOT NULL DEFAULT 0 CHECK (requesting_gid >= 0);
ALTER TABLE commands ADD COLUMN request_message_id TEXT NOT NULL DEFAULT '';
ALTER TABLE commands ADD COLUMN service_uid INTEGER NOT NULL DEFAULT 0 CHECK (service_uid >= 0);
ALTER TABLE commands ADD COLUMN service_username TEXT NOT NULL DEFAULT '';
		`)
		return err
	case 3:
		if err := ensureColumn(tx, "service_uid", "INTEGER NOT NULL DEFAULT 0 CHECK (service_uid >= 0)"); err != nil {
			return err
		}
		return ensureColumn(tx, "service_username", "TEXT NOT NULL DEFAULT ''")
	default:
		return fmt.Errorf("no migration registered for version %d", version)
	}
}

func ensureColumn(tx *sql.Tx, column, declaration string) error {
	rows, err := tx.Query(`PRAGMA table_info(commands)`)
	if err != nil {
		return fmt.Errorf("inspect command ledger columns: %w", err)
	}
	found := false
	for rows.Next() {
		var index, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&index, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return fmt.Errorf("read command ledger column: %w", err)
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read command ledger columns: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close command ledger column query: %w", err)
	}
	if found {
		return nil
	}
	if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE commands ADD COLUMN %s %s", column, declaration)); err != nil {
		return fmt.Errorf("add command ledger column %s: %w", column, err)
	}
	return nil
}

func (l *Ledger) Reconcile(ctx context.Context) error {
	if l == nil || l.db == nil {
		return ErrPersistenceFailed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	if !l.isBlocked() {
		return nil
	}
	if err := l.integrityCheck(); err != nil {
		return l.blockPersistence("reconcile command ledger integrity", err)
	}
	if err := l.recoverInterrupted(); err != nil {
		return l.blockPersistence("reconcile interrupted commands", err)
	}
	var busy, logFrames, checkpointed int
	if err := l.db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(FULL)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		return l.blockPersistence("checkpoint command ledger WAL", err)
	}
	if busy != 0 {
		return l.blockPersistence("checkpoint command ledger WAL", fmt.Errorf("checkpoint busy with %d frames pending", logFrames-checkpointed))
	}
	var unresolved int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM commands WHERE status IN (?, ?, ?)`,
		StatusAuthorizationPending, StatusAccepted, StatusRunning).Scan(&unresolved); err != nil {
		return l.blockPersistence("verify command ledger reconciliation", err)
	}
	if unresolved != 0 {
		return l.blockPersistence("verify command ledger reconciliation", fmt.Errorf("%d unresolved command records remain", unresolved))
	}
	l.gateMu.Lock()
	l.blocked = false
	l.gateMu.Unlock()
	return nil
}

func (l *Ledger) integrityCheck() error {
	rows, err := l.db.Query(`PRAGMA integrity_check`)
	if err != nil {
		return fmt.Errorf("run command ledger integrity check: %w", err)
	}
	defer rows.Close()
	var checks int
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("read command ledger integrity result: %w", err)
		}
		checks++
		if result != "ok" {
			return fmt.Errorf("%w: %s", ErrIntegrityCheck, result)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read command ledger integrity results: %w", err)
	}
	if checks == 0 {
		return fmt.Errorf("%w: no result returned", ErrIntegrityCheck)
	}
	return nil
}

func (l *Ledger) commit(tx *sql.Tx) error {
	if l.commitTransaction != nil {
		return l.commitTransaction(tx)
	}
	return tx.Commit()
}

func (l *Ledger) recoverInterrupted() error {
	_, err := l.db.Exec(`
UPDATE commands
SET status = CASE
	WHEN status IN ('authorization_pending', 'accepted') THEN 'authorization_uncertain'
	WHEN status = 'running' THEN 'unknown'
	ELSE status
END,
updated_at = unixepoch()
WHERE status IN ('authorization_pending', 'accepted', 'running')
`)
	if err != nil {
		return fmt.Errorf("recover interrupted command ledger records: %w", err)
	}
	return nil
}
