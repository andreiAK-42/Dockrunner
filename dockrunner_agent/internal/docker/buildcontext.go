package docker

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
)

func tarDirectory(root string) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		tw := tar.NewWriter(writer)
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			return writeTarEntry(tw, root, path, entry, err)
		})
		_ = tw.Close()
		_ = writer.CloseWithError(err)
	}()
	return reader
}

// writeTarEntry кладёт один файл или папку в tar. .git пропускается целиком.
func writeTarEntry(tw *tar.Writer, root, path string, entry os.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	if entry.IsDir() {
		if entry.Name() == ".git" {
			return filepath.SkipDir
		}
		return writeTarDir(tw, rel, entry)
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	return writeTarFile(tw, path, rel, info)
}

func writeTarDir(tw *tar.Writer, rel string, entry os.DirEntry) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}
	return tw.WriteHeader(&tar.Header{
		Name:     filepath.ToSlash(rel) + "/",
		Mode:     int64(info.Mode().Perm()),
		ModTime:  info.ModTime(),
		Typeflag: tar.TypeDir,
	})
}

func writeTarFile(tw *tar.Writer, path, rel string, info os.FileInfo) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	if err := tw.WriteHeader(&tar.Header{
		Name:     filepath.ToSlash(rel),
		Mode:     int64(info.Mode().Perm()),
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Typeflag: tar.TypeReg,
	}); err != nil {
		return err
	}
	_, err = io.Copy(tw, file)
	return err
}
