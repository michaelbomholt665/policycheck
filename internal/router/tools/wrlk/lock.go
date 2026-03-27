// internal/router/tools/wrlk/lock.go
// Implements the file tracking, snapshotting, and restoration mechanics
// to protect the router core from unplanned drift.

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	routerLockRelativePath     = "internal/router/router.lock"
	routerSnapshotRelativePath = "internal/router/router.snapshot.json"
)

var trackedRouterFiles = []string{
	"internal/router/extension.go",
	"internal/router/registry.go",
	"internal/router/ports.go",
	"internal/router/registry_imports.go",
	"internal/router/router_manifest.go",
	"internal/router/ext/app_manifest.go",
	"internal/router/ext/optional_extensions.go",
	"internal/router/ext/extensions.go",
}

var snapshotRouterFiles = []string{
	"internal/router/extension.go",
	"internal/router/registry.go",
	"internal/router/ports.go",
	"internal/router/registry_imports.go",
	"internal/router/router_manifest.go",
	"internal/router/ext/app_manifest.go",
	"internal/router/ext/optional_extensions.go",
	"internal/router/ext/extensions.go",
	routerLockRelativePath,
}

type lockRecord struct {
	File     string `json:"file"`
	Checksum string `json:"checksum"`
}

// RouterRunLockCommand executes the lock-specific command tree.
func RouterRunLockCommand(options globalOptions, args []string, stdout io.Writer) error {
	if len(args) == 0 || RouterIsHelpToken(args[0]) {
		return RouterWriteLockUsage(stdout)
	}

	switch args[0] {
	case "verify":
		return RouterRunLockVerify(options.root, stdout)
	case "update":
		return RouterRunLockUpdate(options.root, stdout)
	case "restore":
		return RouterRunLockRestore(options.root, stdout)
	default:
		return &usageError{message: fmt.Sprintf("unknown lock subcommand %q", args[0])}
	}
}

// RouterWriteLockUsage prints the lock command usage message.
func RouterWriteLockUsage(writer io.Writer) error {
	lines := []string{
		"usage: Router [--root PATH] lock <subcommand>",
		"subcommands:",
		"  verify   validate tracked router core checksums",
		"  update   rewrite router.lock with current tracked checksums",
		"  restore  restore the previous local router snapshot",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write lock usage line: %w", err)
		}
	}

	return nil
}

// RouterRunLockVerify validates the lock file against the tracked router files.
func RouterRunLockVerify(root string, stdout io.Writer) error {
	records, err := RouterLoadLockRecords(root)
	if err != nil {
		return err
	}

	if err := RouterVerifyTrackedFiles(root, records); err != nil {
		return err
	}

	verifiedFiles := make([]string, 0, len(records))
	for _, record := range records {
		verifiedFiles = append(verifiedFiles, record.File)
	}
	sort.Strings(verifiedFiles)

	if _, err := fmt.Fprintf(stdout, "router lock verified: %s\n", routerLockRelativePath); err != nil {
		return fmt.Errorf("write lock verification status: %w", err)
	}
	for _, relativePath := range verifiedFiles {
		if _, err := fmt.Fprintf(stdout, "verified file: %s\n", relativePath); err != nil {
			return fmt.Errorf("write verified file %s: %w", relativePath, err)
		}
	}

	return nil
}

// RouterRunLockUpdate rewrites the lock file from the tracked router files.
func RouterRunLockUpdate(root string, stdout io.Writer) error {
	records, err := RouterComputeLockRecords(root)
	if err != nil {
		return err
	}

	if err := RouterWriteLockRecords(root, records); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(stdout, "router lock updated: %s\n", routerLockRelativePath); err != nil {
		return fmt.Errorf("write lock update status: %w", err)
	}
	for _, record := range records {
		if _, err := fmt.Fprintf(stdout, "tracked file: %s\n", record.File); err != nil {
			return fmt.Errorf("write tracked file %s: %w", record.File, err)
		}
	}

	return nil
}

// RouterRunLockRestore restores the tracked router snapshot captured before the last mutation.
func RouterRunLockRestore(root string, stdout io.Writer) error {
	snapshot, err := RouterLoadSnapshot(root)
	if err != nil {
		return err
	}

	if err := RouterRestoreSnapshot(root, snapshot); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(stdout, "router snapshot restored: %s\n", routerSnapshotRelativePath); err != nil {
		return fmt.Errorf("write snapshot restore status: %w", err)
	}
	if snapshot.Reason != "" {
		if _, err := fmt.Fprintf(stdout, "snapshot reason: %s\n", snapshot.Reason); err != nil {
			return fmt.Errorf("write snapshot restore reason: %w", err)
		}
	}
	for _, file := range snapshot.Files {
		if _, err := fmt.Fprintf(stdout, "restored file: %s\n", file.File); err != nil {
			return fmt.Errorf("write restored file %s: %w", file.File, err)
		}
	}

	return nil
}

// RouterLoadLockRecords loads and validates lock records from disk.
func RouterLoadLockRecords(root string) ([]lockRecord, error) {
	lockPath := filepath.Join(root, filepath.FromSlash(routerLockRelativePath))
	lockContent, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("router lock verify failed: missing %s", routerLockRelativePath)
		}

		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}

	records := make([]lockRecord, 0, len(trackedRouterFiles))
	lines := strings.Split(string(lockContent), "\n")
	for index, rawLine := range lines {
		line := bytes.TrimSpace([]byte(rawLine))
		if len(line) == 0 {
			continue
		}

		var record lockRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, fmt.Errorf(
				"router lock verify failed: corrupt %s at line %d: %w",
				routerLockRelativePath,
				index+1,
				err,
			)
		}
		if record.File == "" || record.Checksum == "" {
			return nil, fmt.Errorf(
				"router lock verify failed: corrupt %s at line %d",
				routerLockRelativePath,
				index+1,
			)
		}

		records = append(records, record)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("router lock verify failed: corrupt %s", routerLockRelativePath)
	}

	return records, nil
}

// RouterVerifyTrackedFiles compares the on-disk router files against the lock contents.
func RouterVerifyTrackedFiles(root string, records []lockRecord) error {
	expectedRecords, err := RouterComputeLockRecords(root)
	if err != nil {
		return err
	}

	if len(records) != len(expectedRecords) {
		return fmt.Errorf(
			"router lock verify failed: tracked file count mismatch in %s",
			routerLockRelativePath,
		)
	}

	expectedByFile := make(map[string]string, len(expectedRecords))
	for _, record := range expectedRecords {
		expectedByFile[record.File] = record.Checksum
	}

	for _, record := range records {
		expectedChecksum, exists := expectedByFile[record.File]
		if !exists {
			return fmt.Errorf("router lock verify failed: unexpected tracked file %s", record.File)
		}
		if record.Checksum != expectedChecksum {
			return fmt.Errorf("router lock verify failed: checksum mismatch in %s", record.File)
		}
	}

	return nil
}

// RouterComputeLockRecords computes the sorted lock records for all tracked router files.
func RouterComputeLockRecords(root string) ([]lockRecord, error) {
	records := make([]lockRecord, 0, len(trackedRouterFiles))
	for _, relativePath := range trackedRouterFiles {
		checksum, err := RouterChecksumForPath(root, relativePath)
		if err != nil {
			return nil, err
		}

		records = append(records, lockRecord{
			File:     relativePath,
			Checksum: checksum,
		})
	}

	sort.Slice(records, func(i int, j int) bool {
		return records[i].File < records[j].File
	})

	return records, nil
}

// RouterChecksumForPath calculates the content checksum for a tracked file path.
func RouterChecksumForPath(root string, relativePath string) (string, error) {
	absolutePath := filepath.Join(root, filepath.FromSlash(relativePath))
	content, err := os.ReadFile(absolutePath)
	if err != nil {
		return "", fmt.Errorf("read tracked router file %s: %w", relativePath, err)
	}

	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

// RouterWriteLockRecords writes the lock file atomically.
func RouterWriteLockRecords(root string, records []lockRecord) error {
	lockPath := filepath.Join(root, filepath.FromSlash(routerLockRelativePath))
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("create lock directory for %s: %w", lockPath, err)
	}

	var payload bytes.Buffer
	encoder := json.NewEncoder(&payload)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("encode lock record for %s: %w", record.File, err)
		}
	}

	tempFile, err := os.CreateTemp(filepath.Dir(lockPath), "router.lock.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp lock file for %s: %w", lockPath, err)
	}

	tempPath := tempFile.Name()
	writeErr := RouterWriteTempLockFile(tempFile, payload.Bytes())
	if writeErr != nil {
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("write temp lock file %s: %w (cleanup: %v)", tempPath, writeErr, removeErr)
		}

		return writeErr
	}

	if err := os.Rename(tempPath, lockPath); err != nil {
		return fmt.Errorf("replace lock file %s: %w", lockPath, err)
	}

	return nil
}

// RouterWriteTempLockFile flushes one temp lock file to stable storage.
func RouterWriteTempLockFile(file *os.File, payload []byte) error {
	tempPath := file.Name()
	if _, err := file.Write(payload); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("write temp lock file %s: %w (close: %v)", tempPath, err, closeErr)
		}

		return fmt.Errorf("write temp lock file %s: %w", tempPath, err)
	}

	if err := file.Sync(); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("sync temp lock file %s: %w (close: %v)", tempPath, err, closeErr)
		}

		return fmt.Errorf("sync temp lock file %s: %w", tempPath, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp lock file %s: %w", tempPath, err)
	}

	return nil
}

type routerMutationSnapshot struct {
	CreatedAt string                       `json:"created_at"`
	Reason    string                       `json:"reason"`
	Files     []routerMutationSnapshotFile `json:"files"`
}

type routerMutationSnapshotFile struct {
	File     string `json:"file"`
	Exists   bool   `json:"exists"`
	Checksum string `json:"checksum,omitempty"`
	Content  string `json:"content,omitempty"`
}

// RouterWriteSnapshot writes a restorable router snapshot to disk.
func RouterWriteSnapshot(root string, snapshot routerMutationSnapshot) error {
	snapshotPath := filepath.Join(root, filepath.FromSlash(routerSnapshotRelativePath))
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
		return fmt.Errorf("create snapshot directory for %s: %w", snapshotPath, err)
	}

	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode router snapshot %s: %w", routerSnapshotRelativePath, err)
	}
	payload = append(payload, '\n')

	if err := RouterAtomicWriteFile(snapshotPath, payload); err != nil {
		return fmt.Errorf("write router snapshot %s: %w", routerSnapshotRelativePath, err)
	}

	return nil
}

// RouterLoadSnapshot loads the most recent router snapshot from disk.
func RouterLoadSnapshot(root string) (routerMutationSnapshot, error) {
	snapshotPath := filepath.Join(root, filepath.FromSlash(routerSnapshotRelativePath))
	content, err := os.ReadFile(snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			return routerMutationSnapshot{}, fmt.Errorf("router snapshot restore failed: missing %s", routerSnapshotRelativePath)
		}

		return routerMutationSnapshot{}, fmt.Errorf("open router snapshot %s: %w", snapshotPath, err)
	}

	var snapshot routerMutationSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return routerMutationSnapshot{}, fmt.Errorf(
			"router snapshot restore failed: corrupt %s: %w",
			routerSnapshotRelativePath,
			err,
		)
	}
	if len(snapshot.Files) == 0 {
		return routerMutationSnapshot{}, fmt.Errorf(
			"router snapshot restore failed: corrupt %s",
			routerSnapshotRelativePath,
		)
	}

	return snapshot, nil
}

// RouterCaptureSnapshot records the current router file state before mutation.
func RouterCaptureSnapshot(root string, reason string) (routerMutationSnapshot, error) {
	files := make([]routerMutationSnapshotFile, 0, len(snapshotRouterFiles))
	for _, relativePath := range snapshotRouterFiles {
		absolutePath := filepath.Join(root, filepath.FromSlash(relativePath))
		content, err := os.ReadFile(absolutePath)
		if err != nil {
			if os.IsNotExist(err) {
				files = append(files, routerMutationSnapshotFile{File: relativePath, Exists: false})
				continue
			}

			return routerMutationSnapshot{}, fmt.Errorf("read router snapshot file %s: %w", relativePath, err)
		}

		sum := sha256.Sum256(content)
		files = append(files, routerMutationSnapshotFile{
			File:     relativePath,
			Exists:   true,
			Checksum: hex.EncodeToString(sum[:]),
			Content:  string(content),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].File < files[j].File
	})

	return routerMutationSnapshot{
		CreatedAt: RouterSnapshotTimestamp(),
		Reason:    reason,
		Files:     files,
	}, nil
}

// RouterWriteSnapshotBeforeMutation captures and writes a router snapshot before a tooling mutation.
func RouterWriteSnapshotBeforeMutation(root string, reason string) error {
	snapshot, err := RouterCaptureSnapshot(root, reason)
	if err != nil {
		return err
	}

	if err := RouterWriteSnapshot(root, snapshot); err != nil {
		return err
	}

	return nil
}

// RouterRestoreSnapshot restores tracked router files from a prior snapshot.
func RouterRestoreSnapshot(root string, snapshot routerMutationSnapshot) error {
	for _, file := range snapshot.Files {
		absolutePath := filepath.Join(root, filepath.FromSlash(file.File))
		if !file.Exists {
			if err := os.Remove(absolutePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove restored file %s: %w", file.File, err)
			}
			continue
		}

		if err := RouterAtomicWriteFile(absolutePath, []byte(file.Content)); err != nil {
			return fmt.Errorf("restore file %s: %w", file.File, err)
		}
	}

	return nil
}
