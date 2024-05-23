package fs_test

import (
	"io"
	"mr/fs"
	"os"
	"testing"
)

func TestLocalFS(t *testing.T) {
	parent, err := os.MkdirTemp("", "*")
	if err != nil {
		t.Fatal("MkdirTemp:", err)
	}
	defer os.RemoveAll(parent)
	url := "local://" + parent
	fs_, err := fs.NewFileSystem(url)
	if err != nil {
		t.Fatal("NewFileSystem:", url, err)
	}

	fh, err := fs_.Open("data", fs.WRITE)
	if err != nil {
		t.Fatal("fs::Open:", err)
	}
	defer fh.Close()
	n, err := fh.Write([]byte("Hello World!"))
	if err != nil || n != 12 {
		t.Fatal("fh::Write:", err, n)
	}

	info, err := fs_.Stat("data")
	if err != nil || info.Size() != 12 {
		t.Fatal("fs::Stat:", err)
	}

	src, err := fs_.Open("data", fs.READ)
	if err != nil {
		t.Fatal("fs::Open:", err)
	}
	dst, err := fs_.Open("data_copy", fs.WRITE)
	if err != nil {
		t.Fatal("fs::Open:", err)
	}
	m, err := io.Copy(dst, src)
	if err != nil || m != 12 {
		t.Fatal("fh::Write:", err, m)
	}
}
