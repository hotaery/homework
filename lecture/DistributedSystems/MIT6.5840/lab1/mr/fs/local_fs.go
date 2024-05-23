package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type LocalFileSystem struct {
	parentDir string
}

func (fs *LocalFileSystem) Init(url string) (err error) {
	info, err := os.Stat(url)
	if err != nil {
		return
	}
	if !info.IsDir() {
		err = fmt.Errorf("not directory[%s]", url)
		return
	}
	fs.parentDir = url
	return
}

func (fs *LocalFileSystem) normalizePerm(perm Perm) int {
	f := os.O_CREATE
	if perm == READ {
		f |= os.O_RDONLY
	} else {
		f |= os.O_RDWR | os.O_TRUNC
	}
	return f
}

func (fs *LocalFileSystem) Open(fname string, perm Perm) (fh FileHandle, err error) {
	fname = filepath.Join(fs.parentDir, fname)
	fh, err = os.OpenFile(fname, fs.normalizePerm(perm), 0644)
	return
}

func (fs *LocalFileSystem) Stat(fname string) (info os.FileInfo, err error) {
	fname = filepath.Join(fs.parentDir, fname)
	info, err = os.Stat(fname)
	return
}

func (fs *LocalFileSystem) ReadDir(fname string) (entries []os.DirEntry, err error) {
	fname = filepath.Join(fs.parentDir, fname)
	entries, err = os.ReadDir(fname)
	return
}

func (fs *LocalFileSystem) GetLocation(fname string) (Location, error) {
	return nil, errors.ErrUnsupported
}

func (fs *LocalFileSystem) Rename(src string, dst string) error {
	src = filepath.Join(fs.parentDir, src)
	dst = filepath.Join(fs.parentDir, dst)
	return os.Rename(src, dst)
}

func (fs *LocalFileSystem) Unlink(fname string) error {
	fname = filepath.Join(fs.parentDir, fname)
	return os.Remove(fname)
}
