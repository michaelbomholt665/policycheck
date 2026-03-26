// internal/ports/walk.go
// Declares the router port for filesystem tree traversal used by policy checks.
// Keeps walking behavior replaceable in tests and host integrations.
package ports

import "io/fs"

// WalkProvider defines the contract for traversing the filesystem.
type WalkProvider interface {
	// WalkDirectoryTree walks the file tree rooted at root, calling walkFn for each file or directory.
	WalkDirectoryTree(root string, walkFn fs.WalkDirFunc) error
}
