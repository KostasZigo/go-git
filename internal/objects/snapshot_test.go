package objects

import (
	"strings"
	"testing"
)

const validSnapshotHash = "0123456789abcdef0123456789abcdef01234567"

func TestTreeSnapshotValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		snapshot TreeSnapshot
		wantErr  string
	}{
		{
			name: "valid root and nested files",
			snapshot: TreeSnapshot{
				"README.md": {
					Mode: ModeRegularFile,
					Hash: validSnapshotHash,
				},
				"docs/guide.md": {
					Mode: ModeExecutable,
					Hash: validSnapshotHash,
				},
			},
		},
		{
			name: "invalid hash",
			snapshot: TreeSnapshot{
				"README.md": {
					Mode: ModeRegularFile,
					Hash: "not-a-sha-1",
				},
			},
			wantErr: "invalid SHA-1 hash",
		},
		{
			name: "unsupported mode",
			snapshot: TreeSnapshot{
				"README.md": {
					Mode: FileMode("100600"),
					Hash: validSnapshotHash,
				},
			},
			wantErr: "unsupported file mode",
		},
		{
			name: "directory mode",
			snapshot: TreeSnapshot{
				"docs": {
					Mode: ModeDirectory,
					Hash: validSnapshotHash,
				},
			},
			wantErr: "directory mode",
		},
		{
			name: "empty path",
			snapshot: TreeSnapshot{
				"": {
					Mode: ModeRegularFile,
					Hash: validSnapshotHash,
				},
			},
			wantErr: "path cannot be empty",
		},
		{
			name: "absolute POSIX path",
			snapshot: TreeSnapshot{
				"/README.md": {
					Mode: ModeRegularFile,
					Hash: validSnapshotHash,
				},
			},
			wantErr: "path cannot be absolute",
		},
		{
			name: "absolute Windows path",
			snapshot: TreeSnapshot{
				"C:/README.md": {
					Mode: ModeRegularFile,
					Hash: validSnapshotHash,
				},
			},
			wantErr: "path cannot be absolute",
		},
		{
			name: "backslash separator",
			snapshot: TreeSnapshot{
				"docs\\README.md": {
					Mode: ModeRegularFile,
					Hash: validSnapshotHash,
				},
			},
			wantErr: "path cannot contain backslashes",
		},
		{
			name: "current directory segment",
			snapshot: TreeSnapshot{
				"docs/./README.md": {
					Mode: ModeRegularFile,
					Hash: validSnapshotHash,
				},
			},
			wantErr: `path cannot contain "." segments`,
		},
		{
			name: "parent directory segment",
			snapshot: TreeSnapshot{
				"docs/../README.md": {
					Mode: ModeRegularFile,
					Hash: validSnapshotHash,
				},
			},
			wantErr: `path cannot contain ".." segments`,
		},
		{
			name: "empty path segment",
			snapshot: TreeSnapshot{
				"docs//README.md": {
					Mode: ModeRegularFile,
					Hash: validSnapshotHash,
				},
			},
			wantErr: "path cannot contain empty segments",
		},
		{
			name: "trailing slash",
			snapshot: TreeSnapshot{
				"docs/": {
					Mode: ModeRegularFile,
					Hash: validSnapshotHash,
				},
			},
			wantErr: "path cannot have a trailing slash",
		},
		{
			name: "file directory collision",
			snapshot: TreeSnapshot{
				"docs": {
					Mode: ModeRegularFile,
					Hash: validSnapshotHash,
				},
				"docs/README.md": {
					Mode: ModeRegularFile,
					Hash: validSnapshotHash,
				},
			},
			wantErr: "snapshot path collision",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.snapshot.Validate()

			if testCase.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() returned an unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("Validate() error = nil, want an error containing %q", testCase.wantErr)
			}

			if !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("Validate() error = %q, want it to contain %q", err, testCase.wantErr)
			}
		})
	}
}
