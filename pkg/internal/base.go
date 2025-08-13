package internal

import (
	"bufio"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// http client
var httpClient *http.Client

//byte pool
//var bytePool sync.Pool

func init() {
	//http client
	var (
		connectTimeout = 60 * time.Second
		headerTimeout  = 600 * time.Second
		keepAlive      = 60 * time.Second
	)
	httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				//connectTimeout
				Timeout: connectTimeout,
				//keepAlive
				KeepAlive: keepAlive,
			}).DialContext,
			MaxIdleConnsPerHost: 200,
			//keepAlive
			IdleConnTimeout: keepAlive,
			//headerTimeout
			ResponseHeaderTimeout: headerTimeout,
		},
	}

	//byte pool
	//bytePool = sync.Pool{
	//	New: func() any {
	//		buf := make([]byte, 32*1024)
	//		return &buf
	//	},
	//}
}

func CURL(addr, method string, headers map[string]string, body io.Reader, dsc io.Writer, exitChan <-chan bool) (http.Header, error) {
	//readTimeout
	var readTimeout = 600 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	if exitChan != nil {
		go func() {
			for {
				exit, ok := <-exitChan
				if !ok {
					break
				}
				if exit {
					cancel()
					break
				}
			}
		}()
	}
	req, err := http.NewRequest(method, addr, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		if req.Header.Get(k) != "" {
			req.Header.Set(k, v)
		} else {
			req.Header.Add(k, v)
		}
	}
	cl, _ := strconv.ParseInt(req.Header.Get("Content-Length"), 10, 64)
	if cl > 0 {
		req.ContentLength = cl
	}
	resp, err := httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer func() {
		//buf := bytePool.Get().(*[]byte)
		//_, _ = io.CopyBuffer(ioutil.Discard, resp.Body, *buf)
		//bytePool.Put(buf)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if dsc != nil {
		//buf := bytePool.Get().(*[]byte)
		//_, err = io.CopyBuffer(dsc, resp.Body, *buf)
		//bytePool.Put(buf)
		_, err = io.Copy(dsc, resp.Body)
		if err != nil {
			return nil, err
		}
	}
	resp.Header.Set("StatusCode", fmt.Sprintf("%d", resp.StatusCode))
	return resp.Header, nil
	//result := map[string]interface{}{
	//	"StatusCode": resp.StatusCode,
	//}
	//for k, v := range resp.Header {
	//	result[k] = v[0]
	//}
}

// Header http header请求
func Header(addr, method string, headers map[string]string) (http.Header, error) {
	req, err := http.NewRequest(method, addr, strings.NewReader(""))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		if req.Header.Get(k) != "" {
			req.Header.Set(k, v)
		} else {
			req.Header.Add(k, v)
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	//buf := bytePool.Get().(*[]byte)
	//_, _ = io.CopyBuffer(ioutil.Discard, resp.Body, *buf)
	//bytePool.Put(buf)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Header.Set("StatusCode", fmt.Sprintf("%d", resp.StatusCode))
	return resp.Header, nil
	//result := map[string]interface{}{
	//	"StatusCode": resp.StatusCode,
	//}
	//for k, v := range resp.Header {
	//	result[k] = v[0]
	//}
}

func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func Md5Byte(data []byte) []byte {
	m := md5.Sum(data)
	return m[:]
}

func Md5ByteReader(body io.Reader) []byte {
	rs := body.(io.ReadSeeker)
	start, _ := rs.Seek(0, io.SeekCurrent) // Read the whole stream
	defer func() {
		_, _ = rs.Seek(start, io.SeekStart) // Rewind stream at end
	}()
	r := bufio.NewReader(rs)
	hash := md5.New()
	//buf := bytePool.Get().(*[]byte)
	//_, _ = io.CopyBuffer(hash, r, *buf)
	//bytePool.Put(buf)
	_, _ = io.Copy(hash, r)
	return hash.Sum(nil)

	//var buf = make([]byte, bufSize)
	//var hash = md5.New()
	//
	//for {
	//	// Build leaf nodes in 1MB chunks
	//	n, err := r.Read(buf)
	//
	//	if err == io.EOF {
	//		break // This is the last chunk
	//	}
	//
	//	io.Copy(hash, bytes.NewReader(buf[:n]))
	//}
}

func FileIoCopyAt(rs io.ReadSeeker, fd *os.File, off int) (int, error) {
	var size int
	var err error
	_, err = rs.Seek(0, io.SeekStart)
	if err != nil {
		return size, err
	}
	var body []byte
	body, err = io.ReadAll(rs)
	if err != nil {
		return size, err
	}
	size, err = fd.WriteAt(body, int64(off))
	return size, err

	//var buf = make([]byte, bufSize)
	//
	//for {
	//	// Build leaf nodes in 1MB chunks
	//	n, rErr := rs.Read(buf)
	//	if rErr == io.EOF {
	//		break
	//	}
	//	_, err = fd.WriteAt(buf[:n], int64(off))
	//	if err != nil {
	//		break
	//	}
	//	off += n
	//	size += n
	//}
	//return size, nil

	//for {
	//	// Build leaf nodes in 1MB chunks
	//	n, rErr := io.ReadAtLeast(r, buf, bufSize)
	//	if n == 0 {
	//		break
	//	}
	//	_, err = fd.WriteAt(buf[:n], int64(off))
	//	off += n
	//	size += n
	//	if rErr != nil {
	//		break // This is the last chunk
	//	}
	//}
}

func WalkDir(localDir, suffix string) []string {
	var list = make([]string, 0)
	localDir = strings.TrimSuffix(localDir, "/") + "/"
	localDir = filepath.Dir(localDir)
	localDir = strings.Replace(localDir, "\\", "/", -1)
	_ = filepath.Walk(localDir, func(fileName string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi == nil {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		fileName = strings.Replace(fileName, "\\", "/", -1)
		fileName = strings.Replace(fileName, localDir, "", 1)
		fileName = strings.TrimLeft(fileName, "/")

		allowed := true
		if suffix != "" {
			suffixList := strings.Split(suffix, ",")
			for _, tmpSuffix := range suffixList {
				if tmpSuffix == "" {
					continue
				}
				allowed = false
				if strings.HasSuffix(strings.ToLower(fileName), tmpSuffix) {
					allowed = true
					break
				}
			}
		}
		if allowed {
			list = append(list, fileName)
		}
		return nil
	})
	return list
}

// UUID 获取UUID
func UUID() string {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return ""
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	uuidStr := fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])

	return uuidStr
}

// GetDisposition 根据header头获取disposition
func GetDisposition(disposition string) string {
	match := regexp.MustCompile(`filename="(.*)"`).FindStringSubmatch(disposition)
	if len(match) > 0 {
		disposition = match[1]
	}
	return disposition
}
