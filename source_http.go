package main

import (
    "fmt"
    "io/ioutil"
    "net/http"
    "path/filepath"
    "net/url"
    "strconv"
    "regexp"
    "bytes"
    "os"
)

const ImageSourceTypeHttp ImageSourceType = "http"

type HttpImageSource struct {
    Config *SourceConfig
}

func NewHttpImageSource(config *SourceConfig) ImageSource {
    return &HttpImageSource{config}
}

func (s *HttpImageSource) Matches(r *http.Request) bool {
    return r.Method == "GET" && r.URL.Query().Get("url") != ""
}

func (s *HttpImageSource) GetImage(req *http.Request) ([]byte, error) {
    url, err := parseURL(req)
    if err != nil {
        return nil, ErrInvalidImageURL
    }
    if shouldRestrictOrigin(url, s.Config.AllowedOrigings) {
        return nil, fmt.Errorf("Not allowed remote URL origin: %s", url.Host)
    }
    return s.fetchImage(url, req)
}

func (s *HttpImageSource) fetchImage(url *url.URL, ireq *http.Request) ([]byte, error) {

    r := (`(?P<country>pre|gp-[a-z]{2}).*?/(?P<letter>[a-zA-Z]{1})(?P<1number>\d?)(?P<2number>\d?)(?P<3number>\d?)(?P<4number>\d?)(?P<5number>\d?)/?(?P<6number>\d?)/?(?P<imagename>.*)`)
    //debug( "%+v", buildcachepath(r, url.String()))
    //debug( "%+v", s.Config.CacheDirPath)
    fullpath := s.Config.CacheDirPath + "/" + buildcachepath(r, url.String())
    // debug(fullpath);
    if (is_file_cached(fullpath)) {
        buf, err := ioutil.ReadFile(fullpath)
        if err != nil {
            return nil, ErrInvalidFilePath
        }
        debug("Serving cached file.")
        return buf, nil

    } else {
        // Check remote image size by fetching HTTP Headers
        if s.Config.MaxAllowedSize > 0 {
            req := newHTTPRequest(s, ireq, "HEAD", url)
            res, err := http.DefaultClient.Do(req)
            if err != nil {
                return nil, fmt.Errorf("Error fetching image http headers: %v", err)
            }
            res.Body.Close()
            if res.StatusCode < 200 && res.StatusCode > 206 {
                return nil, fmt.Errorf("Error fetching image http headers: (status=%d) (url=%s)", res.StatusCode, req.URL.String())
            }

            contentLength, _ := strconv.Atoi(res.Header.Get("Content-Length"))
            if contentLength > s.Config.MaxAllowedSize {
                return nil, fmt.Errorf("Content-Length %d exceeds maximum allowed %d bytes", contentLength, s.Config.MaxAllowedSize)
            }
        }

        // Perform the request using the default client
        req := newHTTPRequest(s, ireq, "GET", url)
        res, err := http.DefaultClient.Do(req)
        if err != nil {
            return nil, fmt.Errorf("Error downloading image: %v", err)
        }
        defer res.Body.Close()
        if res.StatusCode != 200 {
            return nil, fmt.Errorf("Error downloading image: (status=%d) (url=%s)", res.StatusCode, req.URL.String())
        }

        // Read the body
        buf, err := ioutil.ReadAll(res.Body)
        if err != nil {
            return nil, fmt.Errorf("Unable to create image from response body: %s (url=%s)", req.URL.String(), err)
        }
        c := make(chan int64)
        go cach_file(buf,fullpath,c)
        //     debug("Serving downloaded s3 file: %s", url.String())
        fmt.Printf("Serving downloaded s3 file: %s", url.String())
        return buf, nil
    }
}

func (s *HttpImageSource) setAuthorizationHeader(req *http.Request, ireq *http.Request) {
    auth := s.Config.Authorization
    if auth == "" {
        auth = ireq.Header.Get("X-Forward-Authorization")
    }
    if auth == "" {
        auth = ireq.Header.Get("Authorization")
    }
    if auth != "" {
        req.Header.Set("Authorization", auth)
    }
}

func parseURL(request *http.Request) (*url.URL, error) {
    queryUrl := request.URL.Query().Get("url")
    return url.Parse(queryUrl)
}

func newHTTPRequest(s *HttpImageSource, ireq *http.Request, method string, url *url.URL) *http.Request {
    req, _ := http.NewRequest(method, url.String(), nil)
    req.Header.Set("User-Agent", "imaginary/"+Version)
    req.URL = url

    // Forward auth header to the target server, if necessary
    if s.Config.AuthForwarding || s.Config.Authorization != "" {
        s.setAuthorizationHeader(req, ireq)
    }

    return req
}

func shouldRestrictOrigin(url *url.URL, origins []*url.URL) bool {
    if len(origins) == 0 {
        return false
    }
    for _, origin := range origins {
        if origin.Host == url.Host {
            return false
        }
    }
    return true
}

func init() {
    RegisterSource(ImageSourceTypeHttp, NewHttpImageSource)
}

func getParams(regEx, url string) (paramsMap map[string]string) {

    var compRegEx = regexp.MustCompile(regEx)
    match := compRegEx.FindStringSubmatch(url)

    paramsMap = make(map[string]string)
    for i, name := range compRegEx.SubexpNames() {
        if i > 0 && i <= len(match) {
            paramsMap[name] = match[i]
        }
    }
    return
}

func buildcachepath(regEx, url string) (str string) {

    var compRegEx = regexp.MustCompile(regEx)
    match := compRegEx.FindStringSubmatch(url)

    var returnpath bytes.Buffer

    for i, name := range compRegEx.SubexpNames() {
        if i > 0 && i <= len(match) {
            if match[i] != "" {
                switch name {
                case "country":
/*                    switch match[i] {
                    case "gp-es":
                        returnpath.WriteString("es_ES/")
                    default:
                        */
                        returnpath.WriteString(match[i])
                        returnpath.WriteString("/")
//                    }
                case "letter":
/*                    switch match[i] {
                    case "E":
                        returnpath.WriteString("emp_base/")
                    case "C":
                        returnpath.WriteString("usr/")
                    default:
*/
                        returnpath.WriteString(match[i])
                        returnpath.WriteString("/")
//                    }
                case "imagename":
                    returnpath.WriteString(match[i])
                default:
                    returnpath.WriteString(match[i])
                    returnpath.WriteString("/")
                }
                //debug("%+v", name)
                //			returnpath.WriteString(match[i])
            }
            //debug("%+v", match[i])
        }
    }
    return returnpath.String()
}

func is_file_cached(fullpath string) (present bool) {

    debug(fullpath)
    if _, err := os.Stat(fullpath); os.IsNotExist(err) {
        debug("File does not exists in cache\n")
        return false
    }else{
        debug("File exists in cache\n")
        return true
    }

}

func cach_file(buf []byte, dest string, c chan int64) (error) {

    var fullcachedirpath = filepath.Dir(dest);

    if _, err := os.Stat(fullcachedirpath); os.IsNotExist(err) {
        err = os.MkdirAll(fullcachedirpath, 0770)
        if err != nil {
            fmt.Printf("mkdir recursive operation failed %q\n", err)
        }
    }


    err := ioutil.WriteFile(dest, buf, 0774)

    touchatime(dest)

    if err != nil {
        fmt.Println("Error creating file %s", dest)
        fmt.Println(err)
        return err
    }

    c <- int64(len(buf))
    close(c)

    return nil
}
