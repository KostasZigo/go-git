package index

import "os"

// executableFileMask identifies executable permission bits across user/group/other.
const executableFileMask = 0o111

// DetectFileMode converts os.FileInfo to the corresponding Git index FileMode.
// Checks for symlinks first, then distinguishes executable from regular files.
func DetectFileMode(info os.FileInfo) FileMode {
	mode := info.Mode()

	if mode&os.ModeSymlink != 0 {
		return ModeSymlink
	}

	if mode.IsRegular() {
		if mode.Perm()&executableFileMask != 0 {
			return ModeExecutable
		}
		return ModeRegularFile
	}

	return ModeRegularFile
}
