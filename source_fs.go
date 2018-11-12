package main

import (
	"io/ioutil"
	"net/http"
	"path"
	"strings"
)

const ImageSourceTypeFileSystem ImageSourceType = "fs"

type FileSystemImageSource struct {
	Config *SourceConfig
}

func NewFileSystemImageSource(config *SourceConfig) ImageSource {
	return &FileSystemImageSource{config}
}

func (s *FileSystemImageSource) Matches(r *http.Request) bool {
	return r.Method == "GET" && s.getFileParam(r) != ""
}

func (s *FileSystemImageSource) GetImage(r *http.Request) ([]byte, error) {
	file := s.getFileParam(r)
	if file == "" {
		return nil, ErrMissingParamFile
	}

	file, cache, err := s.buildPath_cache(file)
	if err != nil {
		return nil, err
	}

	if cache != "" {
		c := make(chan int64)
		go defercache(file,cache,c)
	}

	return s.read(file)
}

func (s *FileSystemImageSource) buildPath(file string) (string, error) {
	file = path.Clean(path.Join(s.Config.MountPath, file))
	if strings.HasPrefix(file, s.Config.MountPath) == false {
		return "", ErrInvalidFilePath
	}
	return file, nil
}

func (s *FileSystemImageSource) read(file string) ([]byte, error) {
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, ErrInvalidFilePath
	}
	return buf, nil
}

func (s *FileSystemImageSource) getFileParam(r *http.Request) string {
	return r.URL.Query().Get("file")
}

func init() {
	RegisterSource(ImageSourceTypeFileSystem, NewFileSystemImageSource)
}

func (s *FileSystemImageSource) buildPath_cache(file string) (string, string, error) {
	// first --> return original file or cached file
	// second -> "" if cached file, string if file has to be cached
	// third --> error

    var fullcachedirpathandfile = s.Config.CacheDirPath + file
	file = path.Clean(path.Join(s.Config.MountPath, file))
	cach := ""

	if _, err := os.Stat(fullcachedirpathandfile); os.IsNotExist(err) {
		fmt.Printf("Return original file path\n")
		cach = fullcachedirpathandfile
	}else{
		fmt.Printf("Return cached file path\n")
		file = fullcachedirpathandfile
	}

    fmt.Printf("\nReturn file --> %s\n", file);
		if strings.HasPrefix(file, s.Config.MountPath) == false && strings.HasPrefix(file,s.Config.CacheDirPath) == false {
			return "","", ErrInvalidFilePath
		}

	return file, cach, nil

}

func defercache(src, dst string, c chan int64) () {
	nBytes, err := dofilecache(src, dst)
	if err != nil || nBytes == 0 {
		fmt.Printf("Copy operation to cache failed %q\n", err)
		err := os.Remove(dst)
		if err != nil {
			  fmt.Println(err)
			    return
		}
		//delete file
	} else {
		fmt.Printf("File cached!! (Image Generated: %d bytes, path: %s)\n", nBytes, dst)
	}
	c <- nBytes
	close(c)
}
