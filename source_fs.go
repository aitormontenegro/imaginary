package main

import (
    "io/ioutil"
    "net/http"
    "path"
    "strings"
    "os"
    "fmt"
    "path/filepath"
    "gopkg.in/h2non/bimg.v1"
    "time"
)

const ImageSourceTypeFileSystem ImageSourceType = "fs"

type FileSystemImageSource struct {
    Config *SourceConfig
}

func NewFileSystemImageSource(config *SourceConfig) ImageSource {
    return &FileSystemImageSource{config}
}

func (s *FileSystemImageSource) Matches(r *http.Request) bool {
    if (r.Method == "GET" || r.Method == "HEAD" ) && s.getFileParam(r) != "" {
        return true
    }else{
        return false
    }
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


    if  sourceFileStat, err := os.Stat(fullcachedirpathandfile); err == nil  {
	if destFileStat, err := os.Stat(file); err == nil {

		mtime := sourceFileStat.ModTime()
	        debug("cached --> %+v",mtime)
		mtime2 := destFileStat.ModTime()
		debug("origin --> %+v",mtime2)
		if mtime != mtime2 {
		    debug("File removed")
		    os.Remove(fullcachedirpathandfile)
		}
	}
    }

    if _, err := os.Stat(fullcachedirpathandfile); os.IsNotExist(err) {
        debug("Return original file path\n")
        // Monit pedido por Borja:
        fmt.Printf"Serving file from Isilon: %s\n",file)
        cach = fullcachedirpathandfile

    }else{
        debug("Return cached file path\n")
        touchatime(fullcachedirpathandfile)
        file = fullcachedirpathandfile
    }

    debug("Return file: %s\n", file);
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
	change_mtime(src,dst)
        debug("File cached!! (Image Generated: %d bytes, path: %s)\n", nBytes, dst)
    }
    c <- nBytes
    close(c)
}

func dofilecache(src, dst string) (int64, error) {

     var fullcachedirpath = filepath.Dir(dst);

     if _, err := os.Stat(fullcachedirpath); os.IsNotExist(err) {
        err = os.MkdirAll(fullcachedirpath, 0770)
        if err != nil {
            fmt.Printf("mkdir recursive operation failed %q\n", err)
        }
    }

        source, err := ioutil.ReadFile(src)
        if err != nil {
                return 0, err
        }

    meta, err := bimg.Metadata(source)
     if err != nil {
        return 0, NewError("Cannot retrieve image metadata: %s"+err.Error(), BadRequest)
    }

    var o ImageOptions;
    if meta.Size.Width < 1200 || meta.Size.Height < 840 {
        o.Width = meta.Size.Width
        o.Height = meta.Size.Height
    }else{
        o.Width = 1200;
        o.Height = 840;
    }
        o.Quality = 90;
        o.Colorspace = 22;
        o.StripMetadata = true
        o.Embed = true

        image, err := Fit(source, o)

        var destinationFile = dst
        err = ioutil.WriteFile(destinationFile, image.Body, 0774)

        touchatime(destinationFile)


        if err != nil {
            fmt.Println("Error creating file %s", destinationFile)
            fmt.Println(err)
            return 0, err
        }

        return int64(len(image.Body)), err

}
func touchatime(srcfile string) (error) {


        sourceFileStat, err := os.Stat(srcfile)
        if err != nil {
                return err
        }
        if !sourceFileStat.Mode().IsRegular() {
                return fmt.Errorf("%s is not a regular file", srcfile)
        }
        modifiedtime := sourceFileStat.ModTime()
        os.Chtimes(srcfile, time.Now().Local(), modifiedtime)

        return err

}
func change_mtime(srcfile, destfile string) (error) {

	debug("Change mtime IN")
	debug("%+v",srcfile)
	debug("%+v",destfile)

	sfi, err := os.Stat(srcfile)
	smtime := sfi.ModTime()

        os.Chtimes(destfile, time.Now().Local(), smtime)

	return err

}
