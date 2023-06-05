package common

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Location interface {
	ReplicaGroupId() string
}

type File interface {
	Read(buf []byte) (int, error)
	RandomRead(buf []byte, offset int64) (int, error)
	Append(buf []byte) (int, error)
	Write(buf []byte, offset int64) (int, error)
	Seek(offset int64, whence int) (int64, error)
	Close() error
	GetLocation() (Location, error) // for locality
	Name() string
}

type Directory interface {
	ReadDir(n int) ([]os.DirEntry, error) // stream interface
	CloseDir() error
}

type FileSystemOptions struct {
	RootPath        string
	CreateIfMissing bool
}

type FileSystem interface {
	Init(options FileSystemOptions) error
	Destroy() error
	Create(name string, perm os.FileMode) error
	Open(name string, flags int, perm os.FileMode) (File, error)
	Stat(name string) (os.FileInfo, error)
	Mkdir(name string, perm os.FileMode) error
	Unlink(name string) error
	Rmdir(name string) error
	RecursiveRmdir(name string) error
	OpenDir(name string) (Directory, error)
	Rename(src string, dst string) error
}

type LocalFile struct {
	handle *os.File
}

func (f *LocalFile) Read(buf []byte) (int, error) {
	return f.handle.Read(buf)
}

func (f *LocalFile) RandomRead(buf []byte, offset int64) (int, error) {
	return f.handle.ReadAt(buf, offset)
}

func (f *LocalFile) Append(buf []byte) (int, error) {
	return f.handle.Write(buf)
}

func (f *LocalFile) Write(buf []byte, offset int64) (int, error) {
	return f.handle.WriteAt(buf, offset)
}

func (f *LocalFile) Seek(offset int64, whence int) (int64, error) {
	return f.handle.Seek(offset, whence)
}

func (f *LocalFile) Close() error {
	return f.handle.Close()
}

func (f *LocalFile) GetLocation() (Location, error) {
	err := errors.New("not support")
	return nil, err
}

func (f *LocalFile) Name() string {
	return f.handle.Name()
}

type LocalDirectory struct {
	handle *os.File
}

func (d *LocalDirectory) ReadDir(n int) ([]os.DirEntry, error) {
	return d.handle.ReadDir(n)
}

func (d *LocalDirectory) CloseDir() error {
	return d.handle.Close()
}

type LocalFileSystem struct {
	rootPath string
}

func (fs *LocalFileSystem) Init(options FileSystemOptions) error {
	if len(options.RootPath) == 0 {
		return errors.New("invalid root path")
	}

	fs.rootPath = options.RootPath
	dirInfo, err := os.Stat(fs.rootPath)
	if err != nil {
		if os.IsNotExist(err) && options.CreateIfMissing {
			err = os.Mkdir(fs.rootPath, 0755)
		}
	} else {
		if !dirInfo.IsDir() {
			msg := fmt.Sprintf("%s not directory", fs.rootPath)
			return errors.New(msg)
		}
	}
	return err
}

func (fs *LocalFileSystem) Destroy() error {
	return nil
}

func (fs *LocalFileSystem) Create(name string, perm os.FileMode) error {
	path := filepath.Join(fs.rootPath, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDONLY, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

func (fs *LocalFileSystem) Open(name string, flags int, perm os.FileMode) (File, error) {
	f := &LocalFile{
		handle: nil,
	}

	path := filepath.Join(fs.rootPath, name)
	fh, err := os.OpenFile(path, flags, perm)
	if err != nil {
		return nil, err
	} else {
		f.handle = fh
	}
	return f, err
}

func (fs *LocalFileSystem) Stat(name string) (os.FileInfo, error) {
	path := filepath.Join(fs.rootPath, name)
	return os.Stat(path)
}

func (fs *LocalFileSystem) Mkdir(name string, perm os.FileMode) error {
	path := filepath.Join(fs.rootPath, name)
	return os.Mkdir(path, perm)
}

func (fs *LocalFileSystem) Unlink(name string) error {
	path := filepath.Join(fs.rootPath, name)
	return os.Remove(path)
}

func (fs *LocalFileSystem) Rmdir(name string) error {
	path := filepath.Join(fs.rootPath, name)
	return os.Remove(path)
}

func (fs *LocalFileSystem) RecursiveRmdir(name string) error {
	path := filepath.Join(fs.rootPath, name)
	return os.RemoveAll(path)
}

func (fs *LocalFileSystem) OpenDir(name string) (Directory, error) {
	path := filepath.Join(fs.rootPath, name)
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	} else {
		d := &LocalDirectory{
			handle: fh,
		}
		return d, nil
	}
}

func (fs *LocalFileSystem) Rename(src string, dst string) error {
	srcPath := filepath.Join(fs.rootPath, src)
	dstPath := filepath.Join(fs.rootPath, dst)
	return os.Rename(srcPath, dstPath)
}

func GetLocalFileSystem(rootpath string) (FileSystem, error) {
	options := FileSystemOptions{
		RootPath:        rootpath,
		CreateIfMissing: true,
	}

	fs := &LocalFileSystem{}
	return fs, fs.Init(options)
}
