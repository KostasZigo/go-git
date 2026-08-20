package objects

import (
	"fmt"
	"os"

	"github.com/KostasZigo/gogit/internal/constants"
)

// FileMode represents Unix file permissions and type in Git objects.
type FileMode string

// FileMode constants define the standard Unix permission and type
// values used in Git tree object entries.
const (
	ModeRegularFile FileMode = "100644" // Regular non-executable file
	ModeExecutable  FileMode = "100755" // Executable file
	ModeSymlink     FileMode = "120000" // Symbolic link
	ModeDirectory   FileMode = "040000" // Directory (tree)
	ModeSubmodule   FileMode = "160000" // Git submodule
)

// IsValid verifies file mode matches Git specification.
func (m FileMode) isValid() bool {
	switch m {
	case ModeRegularFile, ModeExecutable, ModeSymlink, ModeDirectory, ModeSubmodule:
		return true
	default:
		return false
	}
}

// ToOsFileMOde converts an objects' package FileMode to the
// corresponding os.FileMode.
func (m FileMode) ToOsFileMOde() (os.FileMode, error) {
	switch m {
	case ModeRegularFile:
		return constants.FilePerms, nil
	case ModeExecutable:
		return constants.ExecutableFilePerms, nil
	case ModeDirectory:
		return constants.DirPerms, nil
	default:
		return 0, fmt.Errorf("unsuported file mode for conversion [%s]", m)
	}
}
