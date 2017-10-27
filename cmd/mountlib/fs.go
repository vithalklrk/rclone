package mountlib

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ncw/rclone/fs"
)

// Node represents either a *Dir or a *File
type Node interface {
	os.FileInfo
	IsFile() bool
	Inode() uint64
	SetModTime(modTime time.Time) error
	Fsync() error
	Remove() error
	RemoveAll() error
	DirEntry() fs.DirEntry
}

var (
	_ Node = (*File)(nil)
	_ Node = (*Dir)(nil)
)

// Nodes is a slice of Node
type Nodes []Node

// Sort functions
func (ns Nodes) Len() int           { return len(ns) }
func (ns Nodes) Swap(i, j int)      { ns[i], ns[j] = ns[j], ns[i] }
func (ns Nodes) Less(i, j int) bool { return ns[i].DirEntry().Remote() < ns[j].DirEntry().Remote() }

// Noder represents something which can return a node
type Noder interface {
	fmt.Stringer
	Node() Node
}

var (
	_ Noder = (*File)(nil)
	_ Noder = (*Dir)(nil)
	_ Noder = (*ReadFileHandle)(nil)
	_ Noder = (*WriteFileHandle)(nil)
)

// FS represents the top level filing system
type FS struct {
	f            fs.Fs
	root         *Dir
	noSeek       bool          // don't allow seeking if set
	noChecksum   bool          // don't check checksums if set
	readOnly     bool          // if set FS is read only
	noModTime    bool          // don't read mod times for files
	dirCacheTime time.Duration // how long to consider directory listing cache valid
}

// NewFS creates a new filing system and root directory
func NewFS(f fs.Fs) *FS {
	fsDir := fs.NewDir("", time.Now())
	fsys := &FS{
		f: f,
	}

	if NoSeek {
		fsys.noSeek = true
	}
	if NoChecksum {
		fsys.noChecksum = true
	}
	if ReadOnly {
		fsys.readOnly = true
	}
	if NoModTime {
		fsys.noModTime = true
	}
	fsys.dirCacheTime = DirCacheTime

	fsys.root = newDir(fsys, f, nil, fsDir)

	if PollInterval > 0 {
		fsys.PollChanges(PollInterval)
	}
	return fsys
}

// PollChanges will poll the remote every pollInterval for changes if the remote
// supports it. If a non-polling option is used, the given time interval can be
// ignored
func (fsys *FS) PollChanges(pollInterval time.Duration) *FS {
	doDirChangeNotify := fsys.f.Features().DirChangeNotify
	if doDirChangeNotify != nil {
		doDirChangeNotify(fsys.root.ForgetPath, pollInterval)
	}
	return fsys
}

// Root returns the root node
func (fsys *FS) Root() (*Dir, error) {
	// fs.Debugf(fsys.f, "Root()")
	return fsys.root, nil
}

var inodeCount uint64

// NewInode creates a new unique inode number
func NewInode() (inode uint64) {
	return atomic.AddUint64(&inodeCount, 1)
}

// Lookup finds the Node by path starting from the root
func (fsys *FS) Lookup(path string) (node Node, err error) {
	node = fsys.root
	for path != "" {
		i := strings.IndexRune(path, '/')
		var name string
		if i < 0 {
			name, path = path, ""
		} else {
			name, path = path[:i], path[i+1:]
		}
		if name == "" {
			continue
		}
		dir, ok := node.(*Dir)
		if !ok {
			// We need to look in a directory, but found a file
			return nil, ENOENT
		}
		node, err = dir.Lookup(name)
		if err != nil {
			return nil, err
		}
	}
	return
}

// Statfs is called to obtain file system metadata.
// It should write that data to resp.
func (fsys *FS) Statfs() error {
	/* FIXME
	const blockSize = 4096
	const fsBlocks = (1 << 50) / blockSize
	resp.Blocks = fsBlocks  // Total data blocks in file system.
	resp.Bfree = fsBlocks   // Free blocks in file system.
	resp.Bavail = fsBlocks  // Free blocks in file system if you're not root.
	resp.Files = 1E9        // Total files in file system.
	resp.Ffree = 1E9        // Free files in file system.
	resp.Bsize = blockSize  // Block size
	resp.Namelen = 255      // Maximum file name length?
	resp.Frsize = blockSize // Fragment size, smallest addressable data size in the file system.
	*/
	return nil
}
