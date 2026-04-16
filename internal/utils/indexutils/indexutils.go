package indexutils

import (
	"os"

	"github.com/KostasZigo/gogit/internal/index"
)

// ExecutableFileMask is used on bitwise opertations for identifying executable files
const ExecutableFileMask = 0111

// detectFileMode converts os.FileInfo mode to Git index FileMode.
func DetectIndexFileMode(info os.FileInfo) index.FileMode {
	mode := info.Mode()

	if mode&os.ModeSymlink != 0 {
		return index.ModeSymlink
	}

	// Check if file is regular (standard file with no special type bits set)
	if mode.IsRegular() {
		// Extract permission bits and check execute flags across user/group/other
		// Non-zero result means at least one execute bit is set
		if mode.Perm()&ExecutableFileMask != 0 {
			return index.ModeExecutable
		}
		return index.ModeRegularFile
	}

	return index.ModeRegularFile
}
