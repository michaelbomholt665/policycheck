// Package embedded Manages the materialization of embedded external scanner scripts.
// Scripts are written to a temporary directory at runtime and cleaned up after use.
//
// Package Concerns:
// - Materialize Python and TypeScript scanner bytes to temp files for subprocess execution.
package embedded
