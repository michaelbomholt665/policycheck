package main

import "time"

// RouterSnapshotTimestamp returns the timestamp used for router snapshots.
func RouterSnapshotTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
