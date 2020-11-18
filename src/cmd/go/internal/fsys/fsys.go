// Package fsys is an abstraction for reading files that
// allows for virtual overlays on top of the files on disk.
package fsys

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// OverlayFile is the path to a text file in the OverlayJSON format.
// It is the value of the -overlay flag.
var OverlayFile string

// OverlayJSON is the format overlay files are expected to be in.
// The Replace map maps from overlaid paths to replacement paths:
// the Go command will forward all reads trying to open
// each overlaid path to its replacement path, or consider the overlaid
// path not to exist if the replacement path is empty.
type OverlayJSON struct {
	Replace map[string]string
}

type node struct {
	actualFilePath string           // empty if a directory
	children       map[string]*node // path element → file or directory
}

func (n *node) isDir() bool {
	return n.actualFilePath == "" && n.children != nil
}

func (n *node) isDeleted() bool {
	return n.actualFilePath == "" && n.children == nil
}

// TODO(matloob): encapsulate these in an io/fs-like interface
var overlay map[string]*node // path -> file or directory node
var cwd string               // copy of base.Cwd to avoid dependency

// Canonicalize a path for looking it up in the overlay.
// Important: filepath.Join(cwd, path) doesn't always produce
// the correct absolute path if path is relative, because on
// Windows producing the correct absolute path requires making
// a syscall. So this should only be used when looking up paths
// in the overlay, or canonicalizing the paths in the overlay.
func canonicalize(path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}

	if v := filepath.VolumeName(cwd); v != "" && path[0] == filepath.Separator {
		// On Windows filepath.Join(cwd, path) doesn't always work. In general
		// filepath.Abs needs to make a syscall on Windows. Elsewhere in cmd/go
		// use filepath.Join(cwd, path), but cmd/go specifically supports Windows
		// paths that start with "\" which implies the path is relative to the
		// volume of the working directory. See golang.org/issue/8130.
		return filepath.Join(v, path)
	}

	// Make the path absolute.
	return filepath.Join(cwd, path)
}

// Init initializes the overlay, if one is being used.
func Init(wd string) error {
	if overlay != nil {
		// already initialized
		return nil
	}

	cwd = wd

	if OverlayFile == "" {
		return nil
	}

	b, err := ioutil.ReadFile(OverlayFile)
	if err != nil {
		return fmt.Errorf("reading overlay file: %v", err)
	}

	var overlayJSON OverlayJSON
	if err := json.Unmarshal(b, &overlayJSON); err != nil {
		return fmt.Errorf("parsing overlay JSON: %v", err)
	}

	return initFromJSON(overlayJSON)
}

func initFromJSON(overlayJSON OverlayJSON) error {
	// Canonicalize the paths in in the overlay map.
	// Use reverseCanonicalized to check for collisions:
	// no two 'from' paths should canonicalize to the same path.
	overlay = make(map[string]*node)
	reverseCanonicalized := make(map[string]string) // inverse of canonicalize operation, to check for duplicates
	// Build a table of file and directory nodes from the replacement map.

	// Remove any potential non-determinism from iterating over map by sorting it.
	replaceFrom := make([]string, 0, len(overlayJSON.Replace))
	for k := range overlayJSON.Replace {
		replaceFrom = append(replaceFrom, k)
	}
	sort.Strings(replaceFrom)

	for _, from := range replaceFrom {
		to := overlayJSON.Replace[from]
		// Canonicalize paths and check for a collision.
		if from == "" {
			return fmt.Errorf("empty string key in overlay file Replace map")
		}
		cfrom := canonicalize(from)
		if to != "" {
			// Don't canonicalize "", meaning to delete a file, because then it will turn into ".".
			to = canonicalize(to)
		}
		if otherFrom, seen := reverseCanonicalized[cfrom]; seen {
			return fmt.Errorf(
				"paths %q and %q both canonicalize to %q in overlay file Replace map", otherFrom, from, cfrom)
		}
		reverseCanonicalized[cfrom] = from
		from = cfrom

		// Create node for overlaid file.
		dir, base := filepath.Dir(from), filepath.Base(from)
		if n, ok := overlay[from]; ok {
			// All 'from' paths in the overlay are file paths. Since the from paths
			// are in a map, they are unique, so if the node already exists we added
			// it below when we create parent directory nodes. That is, that
			// both a file and a path to one of its parent directories exist as keys
			// in the Replace map.
			//
			// This only applies if the overlay directory has any files or directories
			// in it: placeholder directories that only contain deleted files don't
			// count. They are safe to be overwritten with actual files.
			for _, f := range n.children {
				if !f.isDeleted() {
					return fmt.Errorf("invalid overlay: path %v is used as both file and directory", from)
				}
			}
		}
		overlay[from] = &node{actualFilePath: to}

		// Add parent directory nodes to overlay structure.
		childNode := overlay[from]
		for {
			dirNode := overlay[dir]
			if dirNode == nil || dirNode.isDeleted() {
				dirNode = &node{children: make(map[string]*node)}
				overlay[dir] = dirNode
			}
			if childNode.isDeleted() {
				// Only create one parent for a deleted file:
				// the directory only conditionally exists if
				// there are any non-deleted children, so
				// we don't create their parents.
				if dirNode.isDir() {
					dirNode.children[base] = childNode
				}
				break
			}
			if !dirNode.isDir() {
				// This path already exists as a file, so it can't be a parent
				// directory. See comment at error above.
				return fmt.Errorf("invalid overlay: path %v is used as both file and directory", dir)
			}
			dirNode.children[base] = childNode
			parent := filepath.Dir(dir)
			if parent == dir {
				break // reached the top; there is no parent
			}
			dir, base = parent, filepath.Base(dir)
			childNode = dirNode
		}
	}

	return nil
}

// IsDir returns true if path is a directory on disk or in the
// overlay.
func IsDir(path string) (bool, error) {
	path = canonicalize(path)

	if _, ok := parentIsOverlayFile(path); ok {
		return false, nil
	}

	if n, ok := overlay[path]; ok {
		return n.isDir(), nil
	}

	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fi.IsDir(), nil
}

// parentIsOverlayFile returns whether name or any of
// its parents are files in the overlay, and the first parent found,
// including name itself, that's a file in the overlay.
func parentIsOverlayFile(name string) (string, bool) {
	if overlay != nil {
		// Check if name can't possibly be a directory because
		// it or one of its parents is overlaid with a file.
		// TODO(matloob): Maybe save this to avoid doing it every time?
		prefix := name
		for {
			node := overlay[prefix]
			if node != nil && !node.isDir() {
				return prefix, true
			}
			parent := filepath.Dir(prefix)
			if parent == prefix {
				break
			}
			prefix = parent
		}
	}

	return "", false
}

// errNotDir is used to communicate from ReadDir to IsDirWithGoFiles
// that the argument is not a directory, so that IsDirWithGoFiles doesn't
// return an error.
var errNotDir = errors.New("not a directory")

// readDir reads a dir on disk, returning an error that is errNotDir if the dir is not a directory.
// Unfortunately, the error returned by ioutil.ReadDir if dir is not a directory
// can vary depending on the OS (Linux, Mac, Windows return ENOTDIR; BSD returns EINVAL).
func readDir(dir string) ([]os.FileInfo, error) {
	fis, err := ioutil.ReadDir(dir)
	if err == nil {
		return fis, nil
	}

	if os.IsNotExist(err) {
		return nil, err
	} else if dirfi, staterr := os.Stat(dir); staterr == nil && !dirfi.IsDir() {
		return nil, &os.PathError{Op: "ReadDir", Path: dir, Err: errNotDir}
	} else {
		return nil, err
	}
}

// ReadDir provides a slice of os.FileInfo entries corresponding
// to the overlaid files in the directory.
func ReadDir(dir string) ([]os.FileInfo, error) {
	dir = canonicalize(dir)
	if _, ok := parentIsOverlayFile(dir); ok {
		return nil, &os.PathError{Op: "ReadDir", Path: dir, Err: errNotDir}
	}

	dirNode := overlay[dir]
	if dirNode == nil {
		return readDir(dir)
	} else if dirNode.isDeleted() {
		return nil, &os.PathError{Op: "ReadDir", Path: dir, Err: os.ErrNotExist}
	}
	diskfis, err := readDir(dir)
	if err != nil && !os.IsNotExist(err) && !errors.Is(err, errNotDir) {
		return nil, err
	}

	// Stat files in overlay to make composite list of fileinfos
	files := make(map[string]os.FileInfo)
	for _, f := range diskfis {
		files[f.Name()] = f
	}
	for name, to := range dirNode.children {
		switch {
		case to.isDir():
			files[name] = fakeDir(name)
		case to.isDeleted():
			delete(files, name)
		default:
			// This is a regular file.
			f, err := os.Lstat(to.actualFilePath)
			if err != nil {
				files[name] = missingFile(name)
				continue
			} else if f.IsDir() {
				return nil, fmt.Errorf("for overlay of %q to %q: overlay Replace entries can't point to dirctories",
					filepath.Join(dir, name), to.actualFilePath)
			}
			// Add a fileinfo for the overlaid file, so that it has
			// the original file's name, but the overlaid file's metadata.
			files[name] = fakeFile{name, f}
		}
	}
	sortedFiles := diskfis[:0]
	for _, f := range files {
		sortedFiles = append(sortedFiles, f)
	}
	sort.Slice(sortedFiles, func(i, j int) bool { return sortedFiles[i].Name() < sortedFiles[j].Name() })
	return sortedFiles, nil
}

// OverlayPath returns the path to the overlaid contents of the
// file, the empty string if the overlay deletes the file, or path
// itself if the file is not in the overlay, the file is a directory
// in the overlay, or there is no overlay.
// It returns true if the path is overlaid with a regular file
// or deleted, and false otherwise.
func OverlayPath(path string) (string, bool) {
	if p, ok := overlay[canonicalize(path)]; ok && !p.isDir() {
		return p.actualFilePath, ok
	}

	return path, false
}

// Open opens the file at or overlaid on the given path.
func Open(path string) (*os.File, error) {
	cpath := canonicalize(path)
	if node, ok := overlay[cpath]; ok {
		if node.isDir() {
			return nil, &os.PathError{Op: "Open", Path: path, Err: errors.New("fsys.Open doesn't support opening directories yet")}
		}
		return os.Open(node.actualFilePath)
	} else if parent, ok := parentIsOverlayFile(filepath.Dir(cpath)); ok {
		// The file is deleted explicitly in the Replace map,
		// or implicitly because one of its parent directories was
		// replaced by a file.
		return nil, &os.PathError{
			Op:   "Open",
			Path: path,
			Err:  fmt.Errorf("file %s does not exist: parent directory %s is replaced by a file in overlay", path, parent)}
	} else {
		return os.Open(cpath)
	}
}

// IsDirWithGoFiles reports whether dir is a directory containing Go files
// either on disk or in the overlay.
func IsDirWithGoFiles(dir string) (bool, error) {
	fis, err := ReadDir(dir)
	if os.IsNotExist(err) || errors.Is(err, errNotDir) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	var firstErr error
	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}

		// TODO(matloob): this enforces that the "from" in the map
		// has a .go suffix, but the actual destination file
		// doesn't need to have a .go suffix. Is this okay with the
		// compiler?
		if !strings.HasSuffix(fi.Name(), ".go") {
			continue
		}
		if fi.Mode().IsRegular() {
			return true, nil
		}

		// fi is the result of an Lstat, so it doesn't follow symlinks.
		// But it's okay if the file is a symlink pointing to a regular
		// file, so use os.Stat to follow symlinks and check that.
		actualFilePath, _ := OverlayPath(filepath.Join(dir, fi.Name()))
		if fi, err := os.Stat(actualFilePath); err == nil && fi.Mode().IsRegular() {
			return true, nil
		} else if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// No go files found in directory.
	return false, firstErr
}

// walk recursively descends path, calling walkFn. Copied, with some
// modifications from path/filepath.walk.
func walk(path string, info os.FileInfo, walkFn filepath.WalkFunc) error {
	if !info.IsDir() {
		return walkFn(path, info, nil)
	}

	fis, readErr := ReadDir(path)
	walkErr := walkFn(path, info, readErr)
	// If readErr != nil, walk can't walk into this directory.
	// walkErr != nil means walkFn want walk to skip this directory or stop walking.
	// Therefore, if one of readErr and walkErr isn't nil, walk will return.
	if readErr != nil || walkErr != nil {
		// The caller's behavior is controlled by the return value, which is decided
		// by walkFn. walkFn may ignore readErr and return nil.
		// If walkFn returns SkipDir, it will be handled by the caller.
		// So walk should return whatever walkFn returns.
		return walkErr
	}

	for _, fi := range fis {
		filename := filepath.Join(path, fi.Name())
		if walkErr = walk(filename, fi, walkFn); walkErr != nil {
			if !fi.IsDir() || walkErr != filepath.SkipDir {
				return walkErr
			}
		}
	}
	return nil
}

// Walk walks the file tree rooted at root, calling walkFn for each file or
// directory in the tree, including root.
func Walk(root string, walkFn filepath.WalkFunc) error {
	info, err := lstat(root)
	if err != nil {
		err = walkFn(root, nil, err)
	} else {
		err = walk(root, info, walkFn)
	}
	if err == filepath.SkipDir {
		return nil
	}
	return err
}

// lstat implements a version of os.Lstat that operates on the overlay filesystem.
func lstat(path string) (os.FileInfo, error) {
	cpath := canonicalize(path)

	if _, ok := parentIsOverlayFile(filepath.Dir(cpath)); ok {
		return nil, &os.PathError{Op: "lstat", Path: cpath, Err: os.ErrNotExist}
	}

	node, ok := overlay[cpath]
	if !ok {
		// The file or directory is not overlaid.
		return os.Lstat(cpath)
	}

	switch {
	case node.isDeleted():
		return nil, &os.PathError{Op: "lstat", Path: cpath, Err: os.ErrNotExist}
	case node.isDir():
		return fakeDir(filepath.Base(cpath)), nil
	default:
		fi, err := os.Lstat(node.actualFilePath)
		if err != nil {
			return nil, err
		}
		return fakeFile{name: filepath.Base(cpath), real: fi}, nil
	}
}

// fakeFile provides an os.FileInfo implementation for an overlaid file,
// so that the file has the name of the overlaid file, but takes all
// other characteristics of the replacement file.
type fakeFile struct {
	name string
	real os.FileInfo
}

func (f fakeFile) Name() string       { return f.name }
func (f fakeFile) Size() int64        { return f.real.Size() }
func (f fakeFile) Mode() os.FileMode  { return f.real.Mode() }
func (f fakeFile) ModTime() time.Time { return f.real.ModTime() }
func (f fakeFile) IsDir() bool        { return f.real.IsDir() }
func (f fakeFile) Sys() interface{}   { return f.real.Sys() }

// missingFile provides an os.FileInfo for an overlaid file where the
// destination file in the overlay doesn't exist. It returns zero values
// for the fileInfo methods other than Name, set to the file's name, and Mode
// set to ModeIrregular.
type missingFile string

func (f missingFile) Name() string       { return string(f) }
func (f missingFile) Size() int64        { return 0 }
func (f missingFile) Mode() os.FileMode  { return os.ModeIrregular }
func (f missingFile) ModTime() time.Time { return time.Unix(0, 0) }
func (f missingFile) IsDir() bool        { return false }
func (f missingFile) Sys() interface{}   { return nil }

// fakeDir provides an os.FileInfo implementation for directories that are
// implicitly created by overlaid files. Each directory in the
// path of an overlaid file is considered to exist in the overlay filesystem.
type fakeDir string

func (f fakeDir) Name() string       { return string(f) }
func (f fakeDir) Size() int64        { return 0 }
func (f fakeDir) Mode() os.FileMode  { return os.ModeDir | 0500 }
func (f fakeDir) ModTime() time.Time { return time.Unix(0, 0) }
func (f fakeDir) IsDir() bool        { return true }
func (f fakeDir) Sys() interface{}   { return nil }
