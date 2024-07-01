package rprof_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"testing"

	"github.com/polarsignals/rprof"
	"google.golang.org/protobuf/proto"
)

func naiveCopy(dst io.Writer, src io.Reader) error {
	buf := make([]byte, 1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return err
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func TestExternalUsage(t *testing.T) {
	if err := rprof.Start(); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 1024*1024)
	br := bufio.NewReader(rprof.Reader(bytes.NewReader(buf)))
	if err := naiveCopy(io.Discard, br); err != nil {
		t.Fatal(err)
	}

	br = bufio.NewReaderSize(rprof.Reader(bytes.NewReader(buf)), 1024*16)
	if err := naiveCopy(io.Discard, br); err != nil {
		t.Fatal(err)
	}

	prof, err := rprof.Stop()
	if err != nil {
		t.Fatal(err)
	}
	content, err := proto.Marshal(prof)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Create("profile.pb.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	if _, err := gz.Write(content); err != nil {
		t.Fatal(err)
	}
}
