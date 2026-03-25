// internal/ports/walk.go
package ports

import "io/fs"

// WalkProvider defines the contract for traversing the filesystem.
type WalkProvider interface {
	// WalkDirectoryTree walks the file tree rooted at root, calling walkFn for each file or directory.
	WalkDirectoryTree(root string, walkFn fs.WalkDirFunc) error
}
