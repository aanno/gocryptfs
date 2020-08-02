package fusefrontend_reverse

import (
	"context"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/rfjakob/gocryptfs/internal/configfile"
	"github.com/rfjakob/gocryptfs/internal/pathiv"
	"github.com/rfjakob/gocryptfs/internal/syscallcompat"
)

// Node is a file or directory in the filesystem tree
// in a `gocryptfs -reverse` mount.
type Node struct {
	fs.Inode
}

// Lookup - FUSE call for discovering a file.
func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (ch *fs.Inode, errno syscall.Errno) {
	dirfd := int(-1)
	pName := ""
	t := n.lookupFileType(name)
	// gocryptfs.conf
	if t == typeConfig {
		var err error
		rn := n.rootNode()
		dirfd, err = syscallcompat.OpenDirNofollow(rn.args.Cipherdir, "")
		if err != nil {
			errno = fs.ToErrno(err)
			return
		}
		defer syscall.Close(dirfd)
		pName = configfile.ConfReverseName
	} else if t == typeDiriv {
		// gocryptfs.diriv
		dirfd, pName, errno = n.prepareAtSyscall("")
		if errno != 0 {
			return
		}
		defer syscall.Close(dirfd)
		st, err := syscallcompat.Fstatat2(dirfd, pName, unix.AT_SYMLINK_NOFOLLOW)
		if err != nil {
			errno = fs.ToErrno(err)
			return
		}
		content := pathiv.Derive(n.Path(), pathiv.PurposeDirIV)
		var vf *virtualFile
		vf, errno = n.newVirtualFile(content, st, inoTagDirIV)
		if errno != 0 {
			return nil, errno
		}
		out.Attr = vf.attr
		// Create child node
		id := fs.StableAttr{Mode: uint32(vf.attr.Mode), Gen: 1, Ino: vf.attr.Ino}
		ch = n.NewInode(ctx, vf, id)
		return
	} else if t == typeName {
		// gocryptfs.longname.*.name

		// TODO
	} else if t == typeReal {
		// real file
		dirfd, pName, errno = n.prepareAtSyscall(name)
		if errno != 0 {
			return
		}
		defer syscall.Close(dirfd)
	}

	// Get device number and inode number into `st`
	st, err := syscallcompat.Fstatat2(dirfd, pName, unix.AT_SYMLINK_NOFOLLOW)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	// Create new inode and fill `out`
	ch = n.newChild(ctx, st, out)

	if t == typeReal {
		// Translate ciphertext size in `out.Attr.Size` to plaintext size
		n.translateSize(dirfd, pName, &out.Attr)
	}

	return ch, 0
}

// GetAttr - FUSE call for stat()ing a file.
//
// GetAttr is symlink-safe through use of openBackingDir() and Fstatat().
func (n *Node) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) (errno syscall.Errno) {
	// If the kernel gives us a file handle, use it.
	if f != nil {
		return f.(fs.FileGetattrer).Getattr(ctx, out)
	}

	dirfd, pName, errno := n.prepareAtSyscall("")
	if errno != 0 {
		return
	}
	defer syscall.Close(dirfd)

	st, err := syscallcompat.Fstatat2(dirfd, pName, unix.AT_SYMLINK_NOFOLLOW)
	if err != nil {
		return fs.ToErrno(err)
	}

	// Fix inode number
	rn := n.rootNode()
	rn.inoMap.TranslateStat(st)
	out.Attr.FromStat(st)

	// Translate ciphertext size in `out.Attr.Size` to plaintext size
	n.translateSize(dirfd, pName, &out.Attr)

	if rn.args.ForceOwner != nil {
		out.Owner = *rn.args.ForceOwner
	}
	return 0
}