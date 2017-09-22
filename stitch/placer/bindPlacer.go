package placer

import (
	"syscall"

	. "github.com/polydawn/go-errcat"

	"go.polydawn.net/go-timeless-api/rio"
	"go.polydawn.net/rio/fs"
)

var _ Placer = BindPlacer

/*
	Makes files appear in place by use of a bind mount.

	If writable=true, the *source* will be mutable.  If you want the destination
	to be writable, but do not want the source to be mutable, then
	you need a placer like "aufs" or "overlay".
*/
func BindPlacer(srcPath, dstPath fs.AbsolutePath, writable bool) (CleanupFunc, error) {
	// Determine desired type.
	srcStat, err := rootFs.LStat(srcPath.CoerceRelative())
	if err != nil {
		return nil, Errorf(rio.ErrLocalCacheProblem, "error placing with bind mount: %s", err)
	}
	switch srcStat.Type {
	case fs.Type_File: // pass
	case fs.Type_Dir: // pass
	default:
		return nil, Errorf(rio.ErrAssemblyInvalid, "placer: source may only be dir or plain file (%s is %s)", srcPath)
	}

	// Make the destination path exist and be the right type to mount over.
	if err := mkDest(dstPath, srcStat.Type); err != nil {
		return nil, err
	}

	// Make mount syscall to bind, and optionally then push it to readonly.
	//  Works the same for dirs or files.
	flags := syscall.MS_BIND | syscall.MS_REC
	if err := syscall.Mount(srcPath.String(), dstPath.String(), "bind", uintptr(flags), ""); err != nil {
		return nil, Errorf(rio.ErrAssemblyInvalid, "error placing with bind mount: %s", err)
	}
	if !writable {
		flags |= syscall.MS_RDONLY | syscall.MS_REMOUNT
		if err := syscall.Mount(srcPath.String(), dstPath.String(), "bind", uintptr(flags), ""); err != nil {
			return nil, Errorf(rio.ErrAssemblyInvalid, "error placing with bind mount: %s", err)
		}
	}

	// Return a cleanup func that will gracefully unmount.
	return func() error {
		if err := syscall.Unmount(dstPath.String(), 0); err != nil {
			return Errorf(rio.ErrLocalCacheProblem, "error tearing down bind mount: %s", err)
		}
		return nil
	}, nil
}
