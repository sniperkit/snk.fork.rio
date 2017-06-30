package testutil

import (
	"io/ioutil"
	"os"

	"go.polydawn.net/rio/fs"
)

func WithTmpdir(fn func(tmpDir fs.AbsolutePath)) {
	tmpBase := "/tmp/rio-test/"
	err := os.MkdirAll(tmpBase, os.FileMode(0755)|os.ModeSticky)
	if err != nil {
		panic(err)
	}

	tmpdir, err := ioutil.TempDir(tmpBase, "")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(tmpdir)
	fn(fs.MustAbsolutePath(tmpdir))
}