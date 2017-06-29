package osfs

import (
	"os"
	"syscall"

	"go.polydawn.net/rio/fs"
)

func New(basePath fs.AbsolutePath) fs.FS {
	return &osFS{basePath}
}

type osFS struct {
	basePath fs.AbsolutePath
}

func (afs *osFS) BasePath() fs.AbsolutePath {
	return afs.basePath
}

func (afs *osFS) OpenFile(path fs.RelPath, flag int, perms fs.Perms) (fs.File, fs.ErrFS) {
	f, err := os.OpenFile(afs.basePath.Join(path).String(), flag, permsToOs(perms))
	return f, fs.IOError(err)
}

func (afs *osFS) Mkdir(path fs.RelPath, perms fs.Perms) fs.ErrFS {
	err := os.Mkdir(afs.basePath.Join(path).String(), permsToOs(perms))
	return fs.IOError(err)
}

func (afs *osFS) Mklink(path fs.RelPath, target string) fs.ErrFS {
	err := os.Symlink(target, afs.basePath.Join(path).String())
	return fs.IOError(err)
}

func (afs *osFS) Mkfifo(path fs.RelPath, perms fs.Perms) fs.ErrFS {
	err := syscall.Mkfifo(afs.basePath.Join(path).String(), uint32(perms&07777))
	return fs.IOError(err)
}

func (afs *osFS) MkdevBlock(path fs.RelPath, major int64, minor int64, perms fs.Perms) fs.ErrFS {
	mode := uint32(perms&07777) | syscall.S_IFBLK
	err := syscall.Mknod(afs.basePath.Join(path).String(), mode, int(devModesJoin(major, minor)))
	return fs.IOError(err)
}

func (afs *osFS) MkdevChar(path fs.RelPath, major int64, minor int64, perms fs.Perms) fs.ErrFS {
	mode := uint32(perms&07777) | syscall.S_IFCHR
	err := syscall.Mknod(afs.basePath.Join(path).String(), mode, int(devModesJoin(major, minor)))
	return fs.IOError(err)
}

func (afs *osFS) Lchown(path fs.RelPath, uid uint32, gid uint32) fs.ErrFS {
	err := os.Lchown(afs.basePath.Join(path).String(), int(uid), int(gid))
	return fs.IOError(err)
}

func (afs *osFS) Chmod(path fs.RelPath, perms fs.Perms) fs.ErrFS {
	err := os.Chmod(afs.basePath.Join(path).String(), permsToOs(perms))
	return fs.IOError(err)
}

func (afs *osFS) LStat(path fs.RelPath) (*fs.Metadata, fs.ErrFS) {
	fi, err := os.Lstat(afs.basePath.Join(path).String())
	if err != nil {
		return nil, fs.IOError(err)
	}

	// Copy over the easy 1-to-1 parts.
	fmeta := &fs.Metadata{
		Name:  path,
		Size:  fi.Size(),
		Mtime: fi.ModTime(),
	}

	// Munge perms and mode to our types.
	fm := fi.Mode()
	switch fm & (os.ModeType | os.ModeCharDevice) {
	case 0:
		fmeta.Type = fs.Type_File
	case os.ModeDir:
		fmeta.Type = fs.Type_Dir
	case os.ModeSymlink:
		fmeta.Type = fs.Type_Symlink
	case os.ModeNamedPipe:
		fmeta.Type = fs.Type_NamedPipe
	case os.ModeSocket:
		fmeta.Type = fs.Type_Socket
	case os.ModeDevice:
		fmeta.Type = fs.Type_Device
	case os.ModeDevice | os.ModeCharDevice:
		fmeta.Type = fs.Type_CharDevice
	default:
		panic("unknown file mode")
	}
	fmeta.Perms = fs.Perms(fm.Perm())
	if fm&os.ModeSetuid != 0 {
		fmeta.Perms |= fs.Perms_Setuid
	}
	if fm&os.ModeSetgid != 0 {
		fmeta.Perms |= fs.Perms_Setgid
	}
	if fm&os.ModeSticky != 0 {
		fmeta.Perms |= fs.Perms_Sticky
	}

	// Munge UID and GID bits.  These are platform dependent.
	// Also munge device bits if applicable; also platform dependent.
	if sys, ok := fi.Sys().(*syscall.Stat_t); ok {
		fmeta.Uid = sys.Uid
		fmeta.Gid = sys.Gid
		if fmeta.Type == fs.Type_Device || fmeta.Type == fs.Type_CharDevice {
			fmeta.Devmajor, fmeta.Devminor = devModesSplit(sys.Rdev)
		}
	}

	// If it's a symlink, get that info.  It's an extra syscall, but
	//  we almost always want it.
	if target, _, err := afs.Readlink(path); err == nil {
		fmeta.Linkname = target
	} else {
		return nil, err
	}

	// Xattrs are not set by this method, because they require an unbounded
	//  number of additional syscalls (1 to list, $n to get values).

	return fmeta, nil
}

func (afs *osFS) Readlink(path fs.RelPath) (string, bool, fs.ErrFS) {
	target, err := os.Readlink(afs.basePath.Join(path).String())
	switch {
	case err == nil:
		return target, true, nil
	case os.IsNotExist(err):
		return "", false, &fs.ErrNotExists{&path}
	case err.(*os.PathError).Err == syscall.EINVAL:
		// EINVAL means "not a symlink".
		// We return this as false and a nil error because it's frequently useful to use
		// the readlink syscall blindly with an lstat first in order to save a syscall.
		return "", false, nil
	default:
		return "", false, fs.IOError(err)
	}
}

func permsToOs(perms fs.Perms) (mode os.FileMode) {
	mode = os.FileMode(perms & 0777)
	if perms&04000 != 0 {
		mode |= os.ModeSetuid
	}
	if perms&02000 != 0 {
		mode |= os.ModeSetgid
	}
	if perms&01000 != 0 {
		mode |= os.ModeSticky
	}
	return mode
}

func devModesJoin(major int64, minor int64) uint32 {
	// Constants herein are not a joy: they're a workaround for https://github.com/golang/go/issues/8106
	return uint32(((minor & 0xfff00) << 12) | ((major & 0xfff) << 8) | (minor & 0xff))
}

func devModesSplit(rdev uint64) (major int64, minor int64) {
	// Constants herein are not a joy: they're a workaround for https://github.com/golang/go/issues/8106
	return int64((rdev >> 8) & 0xff), int64((rdev & 0xff) | ((rdev >> 12) & 0xfff00))
}