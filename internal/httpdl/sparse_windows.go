package httpdl

import (
	"os"

	"golang.org/x/sys/windows"
)

// makeSparse marks f as an NTFS sparse file. Without this, writing at a
// high offset makes Windows synchronously zero-fill everything before it,
// which turns parallel segment writes into a disk-bound crawl.
func makeSparse(f *os.File) {
	var ret uint32
	_ = windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &ret, nil)
}
