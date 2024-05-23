package fs

import (
	"errors"
	"os"
	"strings"
)

type Perm int

const (
	READ Perm = iota
	WRITE
	RDWR
)

type FileHandle interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type Location interface {
	Addr() string
}

type FileSystem interface {
	Init(url string) error
	Open(string, Perm) (FileHandle, error)
	Stat(string) (os.FileInfo, error)
	ReadDir(string) ([]os.DirEntry, error)
	GetLocation(string) (Location, error)
	Rename(string, string) error
	Unlink(string) error
}

func parseUrl(url string) (protocol string, param string, err error) {
	protocol, param, succ := strings.Cut(url, "://")
	if !succ {
		err = os.ErrInvalid
	}
	return
}

func NewFileSystem(url string) (FileSystem, error) {
	protocol, param, err := parseUrl(url)
	if err != nil {
		return nil, err
	}
	if protocol != "local" {
		return nil, errors.ErrUnsupported
	}
	fs := &LocalFileSystem{}
	if err = fs.Init(param); err != nil {
		return nil, err
	}
	return fs, nil
}
