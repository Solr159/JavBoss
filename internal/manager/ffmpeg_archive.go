package manager

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ulikunitz/xz"
)

// extractFFmpegDownload reads a verified download and copies only the requested
// executable. Archive paths are never used as filesystem destinations.
func extractFFmpegDownload(archive *os.File, target io.Writer, binaryName string) error {
	reader := bufio.NewReader(archive)
	magic, err := reader.Peek(6)
	if err != nil && err != io.EOF {
		return err
	}
	if bytes.HasPrefix(magic, []byte("PK\x03\x04")) {
		info, err := archive.Stat()
		if err != nil {
			return err
		}
		zr, err := zip.NewReader(archive, info.Size())
		if err != nil {
			return err
		}
		for _, entry := range zr.File {
			if !entry.Mode().IsRegular() || !isFFmpegArchiveBinary(entry.Name, binaryName) {
				continue
			}
			file, err := entry.Open()
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(target, file)
			return err
		}
		return fmt.Errorf("archive does not contain bin/%s", binaryName)
	}
	if bytes.HasPrefix(magic, []byte("\xfd7zXZ\x00")) {
		xr, err := xz.NewReader(reader)
		if err != nil {
			return err
		}
		tr := tar.NewReader(xr)
		for {
			entry, err := tr.Next()
			if err == io.EOF {
				return fmt.Errorf("archive does not contain bin/%s", binaryName)
			}
			if err != nil {
				return err
			}
			if entry.Typeflag != tar.TypeReg || !isFFmpegArchiveBinary(entry.Name, binaryName) {
				continue
			}
			_, err = io.Copy(target, tr)
			return err
		}
	}
	if bytes.HasPrefix(magic, []byte{0x1f, 0x8b}) {
		gr, err := gzip.NewReader(reader)
		if err != nil {
			return err
		}
		defer gr.Close()
		_, err = io.Copy(target, gr)
		return err
	}
	_, err = io.Copy(target, reader)
	return err
}

func isFFmpegArchiveBinary(name, binaryName string) bool {
	return name == "bin/"+binaryName || strings.HasSuffix(name, "/bin/"+binaryName)
}
