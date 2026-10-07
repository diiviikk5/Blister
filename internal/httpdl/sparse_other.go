//go:build !windows

package httpdl

import "os"

// makeSparse is a no-op: Unix filesystems create holes on Truncate already.
func makeSparse(*os.File) {}
