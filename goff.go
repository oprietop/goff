package main

import (
    "fmt"
    "os"
    "time"
    "io"
    "io/ioutil"
    "log"
    "net/http"
    "net/url"
    "strings"
    "sync"
    "encoding/json"
    "crypto/md5"
)

const MAX_CONCURRENT_JOBS = 5

// Log error
func errLog(err error) {
    if err != nil {
        log.Printf("Error: %v", err)
    }
}

// Log error and exit 1
func errFail(err error) {
    if err != nil {
        log.Fatalf("Error: %v", err)
    }
}

// Main struct
type RunTasks struct {
    mutex     sync.Mutex
    client    *http.Client
    waitGroup sync.WaitGroup
    waitChan  chan struct{}
    fileUrls  map[string]string
    loopUrls  map[string]string
}

// Needed info for each file
type FileInfo struct {
    uri     string
    id      string
    downUrl string
    name    string
    md5     string
    err     bool
}

// Struct generator
func NewRunTasks() *RunTasks {
    tr := &http.Transport{}
    client := &http.Client{Transport: tr}
    return &RunTasks{
        client: client,
        waitChan: make(chan struct{}, MAX_CONCURRENT_JOBS),
        fileUrls: map[string]string{},
        loopUrls: map[string]string{},
    }
}

// Get the md5 checksum from a file
func (rt *RunTasks) md5(file string) (sum string) {
    fileHandle, err := os.Open(file)
    errLog(err)

    defer fileHandle.Close()

    hash := md5.New()
    _, err = io.Copy(hash, fileHandle)
    errLog(err)

    return fmt.Sprintf("%x", hash.Sum(nil))
 }

// HTTP fetch
func (rt *RunTasks) fetch(uri string) (result []byte) {
    req, _ := http.NewRequest("GET", uri, nil)
    res, err := rt.client.Do(req)
    //res, err := http.Get(uri)
    errLog(err)
    defer res.Body.Close()

    body, err := ioutil.ReadAll(res.Body)
    errLog(err)

    return body
}

// HTTP download to disk
func (rt *RunTasks) download(fi FileInfo) (err error) {
    //res, err := http.Get(uri)
    req, _ := http.NewRequest("GET", fi.downUrl, nil)
    res, err := rt.client.Do(req)
    if err != nil  {
        return err
    }
    defer res.Body.Close()

    if res.StatusCode != http.StatusOK {
        log.Println("Non-OK HTTP status:", res.StatusCode, "fetching", fi.name)
        return fmt.Errorf("%i", res.StatusCode)
    }

    if strings.Contains(res.Header["Content-Type"][0], "down") {
        log.Println("Fetching", fi.name)
    }

    // Create the file and return the handle
    out, err := os.Create(fi.name)
    if err != nil  {
        return err
    }
    defer out.Close()

    // Store the payload in the file handle
    _, err = io.Copy(out, res.Body)
    if err != nil  {
        return err
    }

    // Everything went well
    if strings.Contains(res.Header["Content-Type"][0], "down") {
        if rt.md5(fi.name) != fi.md5 {
            err := os.Remove(fi.name)
            errLog(err)
            return fmt.Errorf("Wrong checksum for '%s', redownloading...", fi.name)
        }
        log.Println("Finished", fi.name)
        return nil
    }

    return fmt.Errorf("%s - %d - %s", fi.name, res.StatusCode, res.Header["Content-Type"][0])
}

// Fetch all the info we need from the link
func (rt *RunTasks) getInfoUri(uri string) (fi FileInfo) {
    fi.uri = uri
    splitUri := strings.Split(uri, "/")
    fi.id = splitUri[len(splitUri)-1]
    uri = "https://api.filefactory.com/v1/getFileInfo?file=" + fi.id
    // Fetch the checksum
    page := rt.fetch(uri)
    var result map[string]interface{}
    json.Unmarshal(page, &result)
    if result["type"] != "success" {
        fi.err = true
        return fi
    }
    fi.md5 = result["result"].
        (map[string]interface{})["files"].
        (map[string]interface{})[fi.id].
        (map[string]interface{})["md5"].
        (string)
    uri = "https://api.filefactory.com/v1/getDownloadLink?file=" + fi.id
    // fetch the download url and filename
    page = rt.fetch(uri)
    json.Unmarshal(page, &result)
    if result["type"] != "success" {
        fi.err = true
        return fi
    }
    fi.downUrl = result["result"].(map[string]interface{})["url"].(string)
    fi.name = result["result"].(map[string]interface{})["name"].(string)

    return fi
}

// Remove finished downloads from urls.txt
func (rt *RunTasks) updateTxt(uri string) (err error) {
    defer rt.mutex.Unlock()
    rt.mutex.Lock()

    delete(rt.fileUrls, uri)

    strUrls := ""
    for u := range rt.fileUrls {
        strUrls = strUrls + u + "\n"
    }

    dataBytes := []byte(strUrls)
    ioutil.WriteFile("urls.txt", dataBytes, 0)

    return nil
}

// Wrapper function to run under goroutines
func (rt *RunTasks) job(uri string) (err error) {
    fi := rt.getInfoUri(uri)
    if fi.err {
        log.Println("Could not find file in", uri, "skipping.")
        return nil
    }
    log.Printf("%+v", fi)
    if _, err := os.Stat(fi.name); err == nil {
        log.Println(fi.name, "exists, skipping.")
    } else {
        // Loop until we got our file
        for {
            err := rt.download(fi)
            //log.Println(downUri, file)
            if err == nil  {
                break
            }
            //log.Println(err)
            time.Sleep(1 * time.Minute)
        }
    }
    // Remove the link from urls.txt
    rt.updateTxt(uri)

    return nil
}

func main() {
    rt := NewRunTasks()

    content, err := ioutil.ReadFile("urls.txt")
    if err != nil {
        errFail(err)
    }
    lines := strings.Split(string(content), "\n")

    for _, line := range lines {
        line = strings.ToLower(line)
        line = strings.TrimSpace(line)
        line = strings.TrimRight(line, "/")
        _, err := url.ParseRequestURI(line)
        if err == nil {
            rt.fileUrls[line] = ""
            rt.loopUrls[line] = ""
        }
    }

    for uri, _ := range rt.loopUrls {
        rt.waitGroup.Add(1)
        rt.waitChan <- struct{}{}
        go func(uri string) {
            defer rt.waitGroup.Done()
            rt.job(uri)
            <-rt.waitChan
        }(uri)
    }

    rt.waitGroup.Wait()
}
