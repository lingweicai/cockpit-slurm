package ledger

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestOpenInitializesVersionedDurableLedger(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	path := filepath.Join(directory, "commands.db")
	ledger, err := Open(path)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()

	var version int
	if err := ledger.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, SchemaVersion)
	}
	var journalMode string
	if err := ledger.db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal mode = %q, want wal", journalMode)
	}
	var synchronous int
	if err := ledger.db.QueryRow(`PRAGMA synchronous`).Scan(&synchronous); err != nil {
		t.Fatalf("read synchronous mode: %v", err)
	}
	if synchronous != 2 {
		t.Fatalf("synchronous mode = %d, want FULL (2)", synchronous)
	}

	directoryInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatalf("stat state directory: %v", err)
	}
	if got := directoryInfo.Mode().Perm(); got != 0750 {
		t.Fatalf("directory mode = %04o, want 0750", got)
	}
	databaseInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat database: %v", err)
	}
	if got := databaseInfo.Mode().Perm(); got != 0600 {
		t.Fatalf("database mode = %04o, want 0600", got)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecarPath := path + suffix
		sidecarInfo, err := os.Stat(sidecarPath)
		if err != nil {
			t.Fatalf("stat SQLite sidecar %s: %v", sidecarPath, err)
		}
		if got := sidecarInfo.Mode().Perm(); got&0077 != 0 {
			t.Fatalf("sidecar %s mode = %04o, want no group/other permissions", suffix, got)
		}
	}
	if ledger.Path() != path {
		t.Fatalf("ledger path = %q, want %q", ledger.Path(), path)
	}
	if err := ledger.integrityCheck(); err != nil {
		t.Fatalf("integrityCheck() returned error: %v", err)
	}
}

func TestOpenRecoversInterruptedCommandsWithoutReexecution(t *testing.T) {
	if seedPath := os.Getenv("COCKPIT_SLURM_LEDGER_SEED_PATH"); seedPath != "" {
		seedInterruptedCommands(t, seedPath)
		return
	}

	if recoveryPath := os.Getenv("COCKPIT_SLURM_LEDGER_RECOVERY_TEST_PATH"); recoveryPath != "" {
		assertRecoveredInterruptedCommands(t, recoveryPath)
		return
	}

	path := filepath.Join(t.TempDir(), "commands.db")
	seedInterruptedCommands(t, path)

	command := exec.Command(os.Args[0], "-test.run=^TestOpenRecoversInterruptedCommandsWithoutReexecution$")
	command.Env = append(os.Environ(), "COCKPIT_SLURM_LEDGER_RECOVERY_TEST_PATH="+path)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("recovery subprocess failed: %v\n%s", err, output)
	}
	if len(output) == 0 {
		t.Fatal("recovery subprocess produced no test output")
	}
}

func seedInterruptedCommands(t *testing.T, path string) {
	t.Helper()
	ledger, err := Open(path)
	if err != nil {
		t.Fatalf("open ledger to seed interrupted commands: %v", err)
	}
	for index, status := range []string{"authorization_pending", "accepted", "running", "succeeded"} {
		commandID := string(rune('a' + index))
		if _, err := ledger.db.Exec(`
INSERT INTO commands (
	caller_uid, command_id, request_fingerprint, request_payload,
	requesting_username, resource, command, operation_id, required_capability,
	target_node, status, created_at, updated_at, expires_at
) VALUES (?, ?, X'01', '{}', 'slurm_test', 'node', 'update', 'node.drain', 'nodes.drain', 'node001', ?, ?, ?, ?)
`, 1001, commandID, status, time.Now().Unix(), time.Now().Unix(), time.Now().Add(Retention).Unix()); err != nil {
			_ = ledger.Close()
			t.Fatalf("insert %s record: %v", status, err)
		}
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("close seeded ledger: %v", err)
	}
}

func assertRecoveredInterruptedCommands(t *testing.T, path string) {
	t.Helper()
	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen ledger: %v", err)
	}
	defer reopened.Close()

	for _, test := range []struct {
		commandID string
		want      string
	}{
		{commandID: "a", want: "authorization_uncertain"},
		{commandID: "b", want: "authorization_uncertain"},
		{commandID: "c", want: "unknown"},
		{commandID: "d", want: "succeeded"},
	} {
		var got string
		if err := reopened.db.QueryRow(`SELECT status FROM commands WHERE caller_uid = ? AND command_id = ?`, 1001, test.commandID).Scan(&got); err != nil {
			t.Fatalf("read command %q status: %v", test.commandID, err)
		}
		if got != test.want {
			t.Errorf("command %q status = %q, want %q", test.commandID, got, test.want)
		}
	}
}

func TestOpenRejectsNewerSchemaAndCorruptDatabase(t *testing.T) {
	t.Run("newer schema", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "commands.db")
		ledger, err := Open(path)
		if err != nil {
			t.Fatalf("Open() returned error: %v", err)
		}
		if _, err := ledger.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, SchemaVersion+1)); err != nil {
			_ = ledger.Close()
			t.Fatalf("set newer schema version: %v", err)
		}
		if err := ledger.Close(); err != nil {
			t.Fatalf("Close() returned error: %v", err)
		}
		if _, err := Open(path); !errors.Is(err, ErrUnsupportedSchema) {
			t.Fatalf("Open() error = %v, want %v", err, ErrUnsupportedSchema)
		}
	})

	t.Run("corrupt database", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "commands.db")
		if err := os.WriteFile(path, []byte("not a sqlite database"), 0600); err != nil {
			t.Fatalf("write corrupt database: %v", err)
		}
		if _, err := Open(path); err == nil {
			t.Fatal("Open() accepted a corrupt database")
		}
	})
}

func TestLookupFailureBlocksCommandAdmission(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close ledger: %v", err)
	}

	if _, _, err := store.Lookup(context.Background(), 1008, "cmd-read-failure"); err == nil {
		t.Fatal("Lookup() succeeded against a closed database")
	}
	if !store.AdmissionBlocked() {
		t.Fatal("ledger admission is not blocked after an operational read failure")
	}
}

func TestReserveAuthorizationIsCallerScopedAndFingerprintSensitive(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()

	firstClaim := testClaim(1008, "cmd-1", []byte(`{"state":"DRAIN"}`))
	first, created, err := ledger.ReserveAuthorization(t.Context(), firstClaim)
	if err != nil || !created || first.Status != StatusAuthorizationPending {
		t.Fatalf("first claim = %#v created=%t err=%v", first, created, err)
	}
	duplicate, created, err := ledger.ReserveAuthorization(t.Context(), firstClaim)
	if err != nil || created || duplicate.Status != StatusAuthorizationPending {
		t.Fatalf("duplicate claim = %#v created=%t err=%v", duplicate, created, err)
	}

	conflicting := testClaim(1008, "cmd-1", []byte(`{"state":"RESUME"}`))
	if _, _, err := ledger.ReserveAuthorization(t.Context(), conflicting); !errors.Is(err, ErrCommandIDConflict) {
		t.Fatalf("conflicting fingerprint error = %v, want %v", err, ErrCommandIDConflict)
	}

	otherCaller := testClaim(1007, "cmd-1", []byte(`{"state":"RESUME"}`))
	other, created, err := ledger.ReserveAuthorization(t.Context(), otherCaller)
	if err != nil || !created || other.CallerUID != 1007 {
		t.Fatalf("other caller claim = %#v created=%t err=%v", other, created, err)
	}
}

func TestCommandLifecycleTransitionsAreDurableAndRestricted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commands.db")
	ledger, err := Open(path)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	claim := testClaim(1008, "cmd-2", []byte(`{"state":"DRAIN"}`))
	if _, _, err := ledger.ReserveAuthorization(t.Context(), claim); err != nil {
		_ = ledger.Close()
		t.Fatalf("ReserveAuthorization() returned error: %v", err)
	}
	metadata := []byte(`{"adminLevel":"admin","capability":"nodes.drain"}`)
	if err := ledger.Transition(t.Context(), 1008, "cmd-2", StatusAuthorizationPending, StatusAccepted, TransitionData{
		AdminLevel:            "admin",
		AuthorizationMetadata: metadata,
	}); err != nil {
		_ = ledger.Close()
		t.Fatalf("pending to accepted: %v", err)
	}
	executionUID := uint32(201)
	if err := ledger.Transition(t.Context(), 1008, "cmd-2", StatusAccepted, StatusRunning, TransitionData{
		ExecutionUID:      &executionUID,
		ExecutionUsername: "cockpit-slurm",
		ExecutionBoundary: "direct-service-account",
	}); err != nil {
		_ = ledger.Close()
		t.Fatalf("accepted to running: %v", err)
	}
	result := []byte(`{"status":"succeeded"}`)
	if err := ledger.Transition(t.Context(), 1008, "cmd-2", StatusRunning, StatusSucceeded, TransitionData{ResultPayload: result}); err != nil {
		_ = ledger.Close()
		t.Fatalf("running to succeeded: %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen ledger: %v", err)
	}
	defer reopened.Close()
	record, found, err := reopened.Lookup(t.Context(), 1008, "cmd-2")
	if err != nil || !found {
		t.Fatalf("Lookup() found=%t err=%v", found, err)
	}
	if record.Status != StatusSucceeded || record.AdminLevel != "admin" || record.ExecutionUID == nil || *record.ExecutionUID != executionUID || record.ExecutionUsername != "cockpit-slurm" || record.ExecutionBoundary != "direct-service-account" || string(record.AuthorizationMetadata) != string(metadata) || string(record.ResultPayload) != string(result) {
		t.Fatalf("recovered record = %#v", record)
	}
	if err := reopened.Transition(t.Context(), 1008, "cmd-2", StatusSucceeded, StatusAccepted, TransitionData{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("terminal status transition error = %v, want %v", err, ErrInvalidTransition)
	}
}

func TestPersistenceFailureBlocksNewClaims(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()
	if _, err := ledger.db.Exec(`DROP TABLE commands`); err != nil {
		t.Fatalf("drop command table for fault injection: %v", err)
	}
	if _, _, err := ledger.ReserveAuthorization(t.Context(), testClaim(1008, "cmd-block", []byte(`{"state":"DRAIN"}`))); !errors.Is(err, ErrPersistenceFailed) {
		t.Fatalf("failed claim error = %v, want %v", err, ErrPersistenceFailed)
	}
	if !ledger.AdmissionBlocked() {
		t.Fatal("admission gate was not blocked after persistence failure")
	}
	if _, _, err := ledger.ReserveAuthorization(t.Context(), testClaim(1008, "cmd-next", []byte(`{"state":"DRAIN"}`))); !errors.Is(err, ErrAdmissionBlocked) {
		t.Fatalf("second claim error = %v, want %v", err, ErrAdmissionBlocked)
	}
	if err := ledger.Reconcile(t.Context()); err == nil {
		t.Fatal("Reconcile() succeeded after the command table was removed")
	}
	if !ledger.AdmissionBlocked() {
		t.Fatal("admission gate opened despite failed reconciliation")
	}
}

func TestCommitBeforeErrorQuarantinesAuthorizationClaim(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()
	ledger.commitTransaction = func(tx *sql.Tx) error {
		if err := tx.Commit(); err != nil {
			return err
		}
		return errors.New("injected error after durable commit")
	}

	if _, _, err := ledger.ReserveAuthorization(t.Context(), testClaim(1008, "cmd-uncertain", []byte(`{"state":"DRAIN"}`))); !errors.Is(err, ErrPersistenceFailed) {
		t.Fatalf("ReserveAuthorization() error = %v, want %v", err, ErrPersistenceFailed)
	}
	if !ledger.AdmissionBlocked() {
		t.Fatal("admission gate is open after uncertain commit")
	}
	if err := ledger.Reconcile(t.Context()); err != nil {
		t.Fatalf("Reconcile() returned error: %v", err)
	}
	if ledger.AdmissionBlocked() {
		t.Fatal("admission gate remained blocked after successful reconciliation")
	}
	record, found, err := ledger.Lookup(t.Context(), 1008, "cmd-uncertain")
	if err != nil || !found || record.Status != StatusAuthorizationUncertain {
		t.Fatalf("reconciled record=%#v found=%t err=%v; want authorization_uncertain", record, found, err)
	}
}

func TestCommitBeforeErrorRunningTransitionBecomesUnknown(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()
	if _, _, err := ledger.ReserveAuthorization(t.Context(), testClaim(1008, "cmd-running", []byte(`{"state":"DRAIN"}`))); err != nil {
		t.Fatalf("ReserveAuthorization() returned error: %v", err)
	}
	if err := ledger.Transition(t.Context(), 1008, "cmd-running", StatusAuthorizationPending, StatusAccepted, TransitionData{
		AdminLevel:            "admin",
		AuthorizationMetadata: []byte(`{"capability":"nodes.drain"}`),
	}); err != nil {
		t.Fatalf("pending to accepted: %v", err)
	}
	ledger.commitTransaction = func(tx *sql.Tx) error {
		if err := tx.Commit(); err != nil {
			return err
		}
		return errors.New("injected error after durable commit")
	}
	executionUID := uint32(201)
	if err := ledger.Transition(t.Context(), 1008, "cmd-running", StatusAccepted, StatusRunning, TransitionData{
		ExecutionUID:      &executionUID,
		ExecutionUsername: "cockpit-slurm",
		ExecutionBoundary: "direct-service-account",
	}); !errors.Is(err, ErrPersistenceFailed) {
		t.Fatalf("accepted to running error = %v, want %v", err, ErrPersistenceFailed)
	}
	if err := ledger.Reconcile(t.Context()); err != nil {
		t.Fatalf("Reconcile() returned error: %v", err)
	}
	record, found, err := ledger.Lookup(t.Context(), 1008, "cmd-running")
	if err != nil || !found || record.Status != StatusUnknown {
		t.Fatalf("reconciled record=%#v found=%t err=%v; want unknown", record, found, err)
	}
}

func TestLifecycleRejectsAcceptanceOrRunningWithoutRequiredMetadata(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()
	if _, _, err := ledger.ReserveAuthorization(t.Context(), testClaim(1008, "cmd-metadata", []byte(`{"state":"DRAIN"}`))); err != nil {
		t.Fatalf("ReserveAuthorization() returned error: %v", err)
	}
	if err := ledger.Transition(t.Context(), 1008, "cmd-metadata", StatusAuthorizationPending, StatusAccepted, TransitionData{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("accept without authorization metadata error = %v, want %v", err, ErrInvalidTransition)
	}
	if err := ledger.Transition(t.Context(), 1008, "cmd-metadata", StatusAuthorizationPending, StatusDenied, TransitionData{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("deny without canonical error payload = %v, want %v", err, ErrInvalidTransition)
	}
	if err := ledger.Transition(t.Context(), 1008, "cmd-metadata", StatusAuthorizationPending, StatusAccepted, TransitionData{
		AdminLevel:            "admin",
		AuthorizationMetadata: []byte(`{"capability":"nodes.drain"}`),
	}); err != nil {
		t.Fatalf("pending to accepted: %v", err)
	}
	if err := ledger.Transition(t.Context(), 1008, "cmd-metadata", StatusAccepted, StatusRunning, TransitionData{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("running without execution identity error = %v, want %v", err, ErrInvalidTransition)
	}
}

func TestCommitBeforeErrorAcceptanceQuarantinesAcceptedRecord(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()
	if _, _, err := ledger.ReserveAuthorization(t.Context(), testClaim(1008, "cmd-accepted", []byte(`{"state":"DRAIN"}`))); err != nil {
		t.Fatalf("ReserveAuthorization() returned error: %v", err)
	}
	ledger.commitTransaction = func(tx *sql.Tx) error {
		if err := tx.Commit(); err != nil {
			return err
		}
		return errors.New("injected error after durable commit")
	}
	if err := ledger.Transition(t.Context(), 1008, "cmd-accepted", StatusAuthorizationPending, StatusAccepted, TransitionData{
		AdminLevel:            "admin",
		AuthorizationMetadata: []byte(`{"capability":"nodes.drain"}`),
	}); !errors.Is(err, ErrPersistenceFailed) {
		t.Fatalf("pending to accepted error = %v, want %v", err, ErrPersistenceFailed)
	}
	if err := ledger.Reconcile(t.Context()); err != nil {
		t.Fatalf("Reconcile() returned error: %v", err)
	}
	record, found, err := ledger.Lookup(t.Context(), 1008, "cmd-accepted")
	if err != nil || !found || record.Status != StatusAuthorizationUncertain {
		t.Fatalf("reconciled record=%#v found=%t err=%v; want authorization_uncertain", record, found, err)
	}
	if err := ledger.Transition(t.Context(), 1008, "cmd-accepted", StatusAuthorizationUncertain, StatusAccepted, TransitionData{
		AdminLevel: "admin", AuthorizationMetadata: []byte(`{"capability":"nodes.drain"}`),
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("uncertain to accepted error = %v, want %v", err, ErrInvalidTransition)
	}
}

func TestCommitBeforeErrorDenialKeepsNonExecutableTombstone(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()
	claim := testClaim(1008, "cmd-denied-uncertain", []byte(`{"state":"DRAIN"}`))
	if _, _, err := ledger.ReserveAuthorization(t.Context(), claim); err != nil {
		t.Fatalf("ReserveAuthorization() returned error: %v", err)
	}
	ledger.commitTransaction = func(tx *sql.Tx) error {
		if err := tx.Commit(); err != nil {
			return err
		}
		return errors.New("injected error after durable commit")
	}
	denial := []byte(`{"code":"authorization-denied","category":"authorization-error","message":"Denied","retryable":false}`)
	if err := ledger.Transition(t.Context(), 1008, claim.CommandID, StatusAuthorizationPending, StatusDenied, TransitionData{ErrorPayload: denial}); !errors.Is(err, ErrPersistenceFailed) {
		t.Fatalf("pending to denied error = %v, want %v", err, ErrPersistenceFailed)
	}
	if err := ledger.Reconcile(t.Context()); err != nil {
		t.Fatalf("Reconcile() returned error: %v", err)
	}
	ledger.commitTransaction = nil
	retry, created, err := ledger.ReserveAuthorization(t.Context(), claim)
	if err != nil || created || retry.Status != StatusDenied {
		t.Fatalf("denial retry=%#v created=%t err=%v, want the stored denial", retry, created, err)
	}
	if err := ledger.Transition(t.Context(), 1008, claim.CommandID, StatusDenied, StatusAccepted, TransitionData{
		AdminLevel: "admin", AuthorizationMetadata: []byte(`{"capability":"nodes.drain"}`),
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("denied to accepted error = %v, want %v", err, ErrInvalidTransition)
	}
}

func TestCommitBeforeErrorLookupFailureKeepsNonExecutableTombstone(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()
	claim := testClaim(1008, "cmd-lookup-failed", []byte(`{"state":"DRAIN"}`))
	if _, _, err := ledger.ReserveAuthorization(t.Context(), claim); err != nil {
		t.Fatalf("ReserveAuthorization() returned error: %v", err)
	}
	ledger.commitTransaction = func(tx *sql.Tx) error {
		if err := tx.Commit(); err != nil {
			return err
		}
		return errors.New("injected error after durable commit")
	}
	lookupError := []byte(`{"code":"user-lookup-failed","category":"internal-error","message":"Lookup failed","retryable":true}`)
	if err := ledger.Transition(t.Context(), 1008, claim.CommandID, StatusAuthorizationPending, StatusAuthorizationFailed, TransitionData{ErrorPayload: lookupError}); !errors.Is(err, ErrPersistenceFailed) {
		t.Fatalf("pending to authorization_failed error = %v, want %v", err, ErrPersistenceFailed)
	}
	if err := ledger.Reconcile(t.Context()); err != nil {
		t.Fatalf("Reconcile() returned error: %v", err)
	}
	ledger.commitTransaction = nil
	retry, created, err := ledger.ReserveAuthorization(t.Context(), claim)
	if err != nil || created || retry.Status != StatusAuthorizationFailed || string(retry.ErrorPayload) != string(lookupError) {
		t.Fatalf("lookup-failure retry=%#v created=%t err=%v, want stored authorization_failed tombstone", retry, created, err)
	}
	if err := ledger.Transition(t.Context(), 1008, claim.CommandID, StatusAuthorizationFailed, StatusAccepted, TransitionData{
		AdminLevel: "admin", AuthorizationMetadata: []byte(`{"capability":"nodes.drain"}`),
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("authorization_failed to accepted error = %v, want %v", err, ErrInvalidTransition)
	}
}

func TestPrecommitFailureReconcilesAbsentClaim(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()
	ledger.commitTransaction = func(*sql.Tx) error { return errors.New("injected pre-commit disk failure") }
	if _, _, err := ledger.ReserveAuthorization(t.Context(), testClaim(1008, "cmd-absent", []byte(`{"state":"DRAIN"}`))); !errors.Is(err, ErrPersistenceFailed) {
		t.Fatalf("ReserveAuthorization() error = %v, want %v", err, ErrPersistenceFailed)
	}
	if !ledger.AdmissionBlocked() {
		t.Fatal("admission gate is open after pre-commit error")
	}
	if err := ledger.Reconcile(t.Context()); err != nil {
		t.Fatalf("Reconcile() returned error: %v", err)
	}
	if ledger.AdmissionBlocked() {
		t.Fatal("admission gate remained blocked after confirming claim absence")
	}
	if _, found, err := ledger.Lookup(t.Context(), 1008, "cmd-absent"); err != nil || found {
		t.Fatalf("claim lookup found=%t err=%v, want absent", found, err)
	}
}

func TestPrecommitTransitionFailuresQuarantineLastDurableState(t *testing.T) {
	for _, test := range []struct {
		name          string
		initialStatus Status
		attempted     Status
		data          TransitionData
	}{
		{
			name:          "denial tombstone",
			initialStatus: StatusAuthorizationPending,
			attempted:     StatusDenied,
			data:          TransitionData{ErrorPayload: []byte(`{"code":"authorization-denied","category":"authorization-error","message":"Denied","retryable":false}`)},
		},
		{
			name:          "accepted reservation",
			initialStatus: StatusAuthorizationPending,
			attempted:     StatusAccepted,
			data:          TransitionData{AdminLevel: "admin", AuthorizationMetadata: []byte(`{"capability":"nodes.drain"}`)},
		},
		{
			name:          "running dispatch intent",
			initialStatus: StatusAccepted,
			attempted:     StatusRunning,
			data: TransitionData{
				ExecutionUID:      uint32Pointer(201),
				ExecutionUsername: "cockpit-slurm",
				ExecutionBoundary: "direct-service-account",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
			if err != nil {
				t.Fatalf("Open() returned error: %v", err)
			}
			defer ledger.Close()
			commandID := "cmd-precommit-" + test.name
			claim := testClaim(1008, commandID, []byte(`{"state":"DRAIN"}`))
			if _, _, err := ledger.ReserveAuthorization(t.Context(), claim); err != nil {
				t.Fatalf("ReserveAuthorization() returned error: %v", err)
			}
			if test.initialStatus == StatusAccepted {
				if err := ledger.Transition(t.Context(), 1008, commandID, StatusAuthorizationPending, StatusAccepted, TransitionData{
					AdminLevel:            "admin",
					AuthorizationMetadata: []byte(`{"capability":"nodes.drain"}`),
				}); err != nil {
					t.Fatalf("pending to accepted: %v", err)
				}
			}
			ledger.commitTransaction = func(*sql.Tx) error { return errors.New("injected pre-commit disk failure") }
			if err := ledger.Transition(t.Context(), 1008, commandID, test.initialStatus, test.attempted, test.data); !errors.Is(err, ErrPersistenceFailed) {
				t.Fatalf("transition error = %v, want %v", err, ErrPersistenceFailed)
			}
			ledger.commitTransaction = nil
			if err := ledger.Reconcile(t.Context()); err != nil {
				t.Fatalf("Reconcile() returned error: %v", err)
			}
			record, found, err := ledger.Lookup(t.Context(), 1008, commandID)
			if err != nil || !found || record.Status != StatusAuthorizationUncertain {
				t.Fatalf("reconciled record=%#v found=%t err=%v; want authorization_uncertain", record, found, err)
			}
		})
	}
}

func TestConcurrentIdenticalClaimsHaveOneWinner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commands.db")
	ledger, err := Open(path)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()
	secondLedger, err := Open(path)
	if err != nil {
		t.Fatalf("open second ledger handle: %v", err)
	}
	defer secondLedger.Close()
	ledgers := []*Ledger{ledger, secondLedger}
	claim := testClaim(1008, "cmd-race", []byte(`{"state":"DRAIN"}`))
	const attempts = 24
	var workers sync.WaitGroup
	var resultMu sync.Mutex
	createdCount := 0
	errorsFound := make(chan error, attempts)
	for index := range attempts {
		workers.Add(1)
		go func(ledger *Ledger) {
			defer workers.Done()
			_, created, err := ledger.ReserveAuthorization(t.Context(), claim)
			if err != nil {
				errorsFound <- err
				return
			}
			if created {
				resultMu.Lock()
				createdCount++
				resultMu.Unlock()
			}
		}(ledgers[index%len(ledgers)])
	}
	workers.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("ReserveAuthorization() error: %v", err)
	}
	if createdCount != 1 {
		t.Fatalf("successful claim owners = %d, want 1", createdCount)
	}
}

func TestDatabaseLockBlocksAndReconcilesAdmission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commands.db")
	writer, err := Open(path)
	if err != nil {
		t.Fatalf("Open(writer) returned error: %v", err)
	}
	defer writer.Close()
	contender, err := Open(path)
	if err != nil {
		t.Fatalf("Open(contender) returned error: %v", err)
	}
	defer contender.Close()
	if _, err := contender.db.Exec(`PRAGMA busy_timeout = 50`); err != nil {
		t.Fatalf("set contender busy timeout: %v", err)
	}

	tx, err := writer.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin writer transaction: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO commands (
	caller_uid, command_id, request_fingerprint, request_payload,
	requesting_username, resource, command, operation_id, required_capability,
	target_node, status, created_at, updated_at, expires_at
) VALUES (1008, 'cmd-uncommitted', X'01', '{}', 'slurm_admin', 'node', 'update', 'node.drain', 'nodes.drain', 'node001', 'authorization_pending', 1, 1, 9999999999)`); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert uncommitted writer row: %v", err)
	}

	if _, _, err := contender.ReserveAuthorization(t.Context(), testClaim(1007, "cmd-locked", []byte(`{"state":"DRAIN"}`))); !errors.Is(err, ErrPersistenceFailed) {
		_ = tx.Rollback()
		t.Fatalf("contending claim error = %v, want %v", err, ErrPersistenceFailed)
	}
	if !contender.AdmissionBlocked() {
		_ = tx.Rollback()
		t.Fatal("contending ledger did not block admission")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback writer transaction: %v", err)
	}
	if err := contender.Reconcile(t.Context()); err != nil {
		t.Fatalf("Reconcile() returned error after lock release: %v", err)
	}
	if contender.AdmissionBlocked() {
		t.Fatal("admission remained blocked after confirming absent claim")
	}
	if _, found, err := contender.Lookup(t.Context(), 1007, "cmd-locked"); err != nil || found {
		t.Fatalf("contended claim found=%t err=%v, want absent", found, err)
	}
}

func TestExpiredCommandIDCanBeReservedAfterRetention(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()
	oldClaim := testClaim(1008, "cmd-expired", []byte(`{"state":"DRAIN"}`))
	if _, _, err := ledger.ReserveAuthorization(t.Context(), oldClaim); err != nil {
		t.Fatalf("initial claim: %v", err)
	}
	if _, err := ledger.db.Exec(`UPDATE commands SET expires_at = ? WHERE caller_uid = ? AND command_id = ?`, time.Now().Add(-time.Second).Unix(), 1008, "cmd-expired"); err != nil {
		t.Fatalf("expire old record: %v", err)
	}
	if _, found, err := ledger.Lookup(t.Context(), 1008, "cmd-expired"); err != nil || found {
		t.Fatalf("expired lookup found=%t err=%v, want absent", found, err)
	}
	newClaim := testClaim(1008, "cmd-expired", []byte(`{"state":"RESUME"}`))
	if _, created, err := ledger.ReserveAuthorization(t.Context(), newClaim); err != nil || !created {
		t.Fatalf("reserve expired ID created=%t err=%v, want a new reservation", created, err)
	}
}

func TestOpenPurgesExpiredCommandRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commands.db")
	ledger, err := Open(path)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	claim := testClaim(1008, "cmd-purge", []byte(`{"state":"DRAIN"}`))
	if _, _, err := ledger.ReserveAuthorization(t.Context(), claim); err != nil {
		_ = ledger.Close()
		t.Fatalf("ReserveAuthorization() returned error: %v", err)
	}
	if _, err := ledger.db.Exec(`UPDATE commands SET expires_at = ? WHERE caller_uid = ? AND command_id = ?`, time.Now().Add(-time.Second).Unix(), claim.CallerUID, claim.CommandID); err != nil {
		_ = ledger.Close()
		t.Fatalf("expire command record: %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen ledger: %v", err)
	}
	defer reopened.Close()
	var count int
	if err := reopened.db.QueryRow(`SELECT COUNT(*) FROM commands WHERE caller_uid = ? AND command_id = ?`, claim.CallerUID, claim.CommandID).Scan(&count); err != nil {
		t.Fatalf("count expired record: %v", err)
	}
	if count != 0 {
		t.Fatalf("expired rows = %d, want 0", count)
	}
}

func TestDeniedTombstoneCannotBecomeAccepted(t *testing.T) {
	ledger, err := Open(filepath.Join(t.TempDir(), "commands.db"))
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer ledger.Close()
	claim := testClaim(1008, "cmd-denied", []byte(`{"state":"DRAIN"}`))
	if _, _, err := ledger.ReserveAuthorization(t.Context(), claim); err != nil {
		t.Fatalf("ReserveAuthorization() returned error: %v", err)
	}
	denial := []byte(`{"code":"authorization-denied","category":"authorization-error","message":"Denied","retryable":false}`)
	if err := ledger.Transition(t.Context(), 1008, "cmd-denied", StatusAuthorizationPending, StatusDenied, TransitionData{ErrorPayload: denial}); err != nil {
		t.Fatalf("pending to denied: %v", err)
	}
	retry, created, err := ledger.ReserveAuthorization(t.Context(), claim)
	if err != nil || created || retry.Status != StatusDenied {
		t.Fatalf("denied retry = %#v created=%t err=%v", retry, created, err)
	}
	if err := ledger.Transition(t.Context(), 1008, "cmd-denied", StatusDenied, StatusAccepted, TransitionData{
		AdminLevel: "admin", AuthorizationMetadata: []byte(`{"capability":"nodes.drain"}`),
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("denied to accepted error = %v, want %v", err, ErrInvalidTransition)
	}
}

func testClaim(callerUID uint32, commandID string, payload []byte) Claim {
	fingerprint := sha256.Sum256(payload)
	return Claim{
		CallerUID:          callerUID,
		CommandID:          commandID,
		RequestFingerprint: fingerprint[:],
		RequestPayload:     payload,
		RequestingUsername: "slurm_test",
		Resource:           "node",
		Command:            "update",
		OperationID:        "node.drain",
		RequiredCapability: "nodes.drain",
		TargetNode:         "node001",
	}
}

func uint32Pointer(value uint32) *uint32 {
	return &value
}
