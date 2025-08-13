package v4

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shideqin/go-s3-sdk/pkg/internal"
	"github.com/shideqin/go-s3-sdk/pkg/s3"
)

// CopyObjectResult COPY结果
type CopyObjectResult = s3.CopyObjectResult

// ListObjectResult 列表结果
type ListObjectResult = s3.ListObjectResult

// ListObjectPrefixes 列表前缀
type ListObjectPrefixes = s3.ListObjectPrefixes

// ListObjectContents 列表内容
type ListObjectContents = s3.ListObjectContents

// UploadFile 上传文件根据路径
func (c *Client) UploadFile(filePath, bucket, object string, options map[string]string) (map[string]interface{}, error) {
	fd, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf(" UploadFile Open localFile: %s Error: %v", filePath, err)
	}
	defer fd.Close()
	stat, err := fd.Stat()
	if err != nil {
		return nil, fmt.Errorf(" UploadFile Stat localFile: %s Error: %v", filePath, err)
	}
	bodySize := int(stat.Size())
	if object == "" {
		object = path.Base(filePath)
	}
	if strings.TrimSuffix(object, "/") == path.Dir(object) {
		object = path.Dir(object) + "/" + path.Base(filePath)
	}
	return c.Put(fd, bodySize, bucket, object, map[string]string{"disposition": options["disposition"], "acl": options["acl"]})
}

// Put 上传文件根据内容
func (c *Client) Put(content io.Reader, bodySize int, bucket, object string, options map[string]string) (map[string]interface{}, error) {
	nObject := url.QueryEscape(object)
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/%s", host, nObject)
	method := "PUT"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	contentSha256 := hex.EncodeToString(hashSHA256Reader(content))
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": contentSha256,
	}
	if options["acl"] != "" {
		headers["x-amz-acl"] = options["acl"]
	}
	headers["Authorization"] = c.sign(method, headers, "/"+nObject, "")
	if options["disposition"] != "" {
		headers["Content-Disposition"] = fmt.Sprintf(`attachment; filename="%s"`, options["disposition"])
	}
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, content, body, nil)
	if err != nil {
		return nil, fmt.Errorf(" Put Object: %s Error: %v", object, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" Put Object: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", object, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	return map[string]interface{}{
		"X-Amz-Request-Id": reqID,
		"StatusCode":       status,
		"Location":         fmt.Sprintf("http://%s.%s/%s", bucket, c.host, object),
		"Size":             bodySize,
		"Bucket":           bucket,
		"ETag":             header.Get("Etag"),
		"Key":              object,
	}, nil
}

// Copy 复制文件
func (c *Client) Copy(bucket, object, source string, options map[string]string) (map[string]interface{}, error) {
	//source head
	tmpSourceInfo := strings.Split(source, "/")
	sourceBucket := tmpSourceInfo[1]
	sourceObject := strings.Join(tmpSourceInfo[2:], "/")
	sourceHead, err := c.Head(sourceBucket, sourceObject)
	if err != nil {
		return nil, err
	}
	if object == "" {
		object = path.Base(sourceObject)
	}
	nObject := url.QueryEscape(object)
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/%s", host, nObject)
	method := "PUT"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
		"x-amz-copy-source":    source,
	}
	if options["acl"] != "" {
		headers["x-amz-acl"] = options["acl"]
	}
	headers["Authorization"] = c.sign(method, headers, "/"+nObject, "")
	if options["disposition"] != "" {
		headers["response-content-disposition"] = fmt.Sprintf(`attachment; filename="%s"`, options["disposition"])
	}
	body := &bytes.Buffer{}
	header, cErr := internal.CURL(addr, method, headers, strings.NewReader(""), body, nil)
	if cErr != nil {
		return nil, fmt.Errorf(" Copy Object: %s Error: %v", object, cErr)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" Copy Object: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", object, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	var CopyObject = &CopyObjectResult{}
	if err = xml.Unmarshal(body.Bytes(), CopyObject); err != nil {
		return nil, fmt.Errorf(" Copy Object: %s Error: %v", object, err)
	}
	var contentLength, _ = strconv.Atoi(sourceHead.Get("Content-Length"))
	return map[string]interface{}{
		"X-Amz-Request-Id": reqID,
		"StatusCode":       status,
		"Location":         fmt.Sprintf("http://%s.%s/%s", bucket, c.host, object),
		"Size":             contentLength,
		"Bucket":           bucket,
		"ETag":             CopyObject.ETag,
		"Key":              object,
	}, nil
}

// Delete 删除文件
func (c *Client) Delete(bucket, object string) (http.Header, error) {
	nObject := url.QueryEscape(object)
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/%s", host, nObject)
	method := "DELETE"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	headers["Authorization"] = c.sign(method, headers, "/"+nObject, "")
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), nil, nil)
	if err != nil {
		return nil, fmt.Errorf(" Delete Object: %s Error: %v", object, err)
	}
	return header, nil
}

// Head 查看文件信息
func (c *Client) Head(bucket, object string) (http.Header, error) {
	nObject := url.QueryEscape(object)
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/%s", host, nObject)
	method := "HEAD"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	headers["Authorization"] = c.sign(method, headers, "/"+nObject, "")
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), nil, nil)
	if err != nil {
		return nil, fmt.Errorf(" Head Object: %s Error: %v", object, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		return nil, fmt.Errorf(" Head Object: %s StatusCode: %s X-Amz-Request-Id: %s", object, status, reqID)
	}
	return header, nil
}

// Get 下载文件到本地
func (c *Client) Get(bucket, object, localFile string, options map[string]string, percentChan chan int) (map[string]string, error) {
	objectHead, headErr := c.Head(bucket, object)
	if headErr != nil {
		return nil, headErr
	}
	var objectSize, _ = strconv.Atoi(objectHead.Get("Content-Length"))
	//当没指定文件名时，默认使用object的文件名
	if strings.TrimSuffix(localFile, "/") == path.Dir(localFile) {
		localFile = path.Dir(localFile) + "/" + path.Base(object)
	}
	var partSize = c.partMinSize
	if options["part_size"] != "" {
		n, _ := strconv.Atoi(options["part_size"])
		if n <= c.partMaxSize && n >= c.partMinSize {
			partSize = n
		}
	}
	var threadNum = c.threadMaxNum
	if options["thread_num"] != "" {
		n, _ := strconv.Atoi(options["thread_num"])
		if n <= c.threadMaxNum && n >= c.threadMinNum {
			threadNum = n
		}
	}

	//创建local文件
	var localDir = path.Dir(localFile)
	err := os.MkdirAll(localDir, 0666)
	if err != nil {
		return nil, fmt.Errorf(" Get MkdirAll LocalDir: %s Error: %v", localDir, err)
	}
	//file, oErr := os.OpenFile(localFile, os.O_CREATE|os.O_WRONLY, 0755)
	file, oErr := os.Create(localFile)
	if oErr != nil {
		return nil, fmt.Errorf(" Get OpenFile localFile: %s Error: %v", localFile, oErr)
	}
	defer file.Close()
	var total = (objectSize + partSize - 1) / partSize
	var queueMaxSize = make(chan bool, threadNum)
	defer close(queueMaxSize)
	var partErr error
	var partExit bool
	var wg sync.WaitGroup
	for partNum := 0; partNum < total; partNum++ {
		if partExit {
			break
		}
		wg.Add(1)
		queueMaxSize <- true
		go func(partNum int) {
			defer func() {
				if partErr != nil {
					partExit = true
				}
				wg.Done()
				<-queueMaxSize
			}()
			tFile, tErr := os.CreateTemp("", fmt.Sprintf("aws-v4-get%d", partNum))
			if tErr != nil {
				partErr = tErr
				return
			}
			defer func() {
				tFile.Close()
				os.Remove(tFile.Name())
			}()
			//part范围,如：0-1023
			tmpStart := partNum * partSize
			tmpEnd := (partNum+1)*partSize - 1
			if tmpEnd > objectSize {
				tmpEnd = tmpStart + objectSize%partSize - 1
			}
			partRange := fmt.Sprintf("bytes=%d-%d", tmpStart, tmpEnd)
			for i := 0; i < c.maxRetryNum; i++ {
				_, sErr := tFile.Seek(0, io.SeekStart)
				if sErr != nil {
					partErr = sErr
					continue
				}
				_, cErr := c.Cat(bucket, object, partRange, tFile)
				if cErr != nil {
					partErr = cErr
					continue
				}
				partErr = nil
				break
			}
			if partErr != nil {
				return
			}
			_, wErr := internal.FileIoCopyAt(tFile, file, tmpStart)
			if wErr != nil {
				partErr = wErr
				return
			}
			if percentChan != nil {
				percentChan <- total
			}
		}(partNum)
	}
	wg.Wait()
	if partErr != nil {
		return nil, partErr
	}
	return map[string]string{"Object": object, "Localfile": localFile}, nil
}

// Cat 读取文件内容
func (c *Client) Cat(bucket, object, partRange string, dsc io.Writer) (http.Header, error) {
	nObject := url.QueryEscape(object)
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/%s", host, nObject)
	method := "GET"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	headers["Authorization"] = c.sign(method, headers, "/"+nObject, "")
	//分片请求
	if partRange != "" {
		headers["Range"] = partRange
	}
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), dsc, nil)
	if err != nil {
		return nil, fmt.Errorf(" Cat Object: %s Error: %v", object, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" && status != "206" {
		return nil, fmt.Errorf(" Cat Object: %s StatusCode: %s X-Amz-Request-Id: %s", object, status, reqID)
	}
	return header, nil
}

// UploadFromDir 上传目录
func (c *Client) UploadFromDir(localDir, bucket, prefix string, options map[string]string, percentChan chan int) (map[string]int, error) {
	if prefix != "" {
		prefix = strings.TrimSuffix(prefix, "/") + "/"
	}
	suffix := ""
	if options["suffix"] != "" {
		suffix = options["suffix"]
	}
	localDir = strings.TrimSuffix(localDir, "/") + "/"
	fileList := internal.WalkDir(localDir, suffix)
	total := len(fileList)
	var threadNum = c.threadMaxNum
	if options["thread_num"] != "" {
		n, _ := strconv.Atoi(options["thread_num"])
		if n <= c.threadMaxNum && n >= c.threadMinNum {
			threadNum = n
		}
	}
	if total < threadNum {
		threadNum = total
	}

	var queueMaxSize = make(chan bool, threadNum)
	defer close(queueMaxSize)
	var fileErr error
	var fileExit bool
	var tmpSize int64
	var tmpSkip int64
	var tmpFinish int64
	var wg sync.WaitGroup
	for fileNum := 0; fileNum < total; fileNum++ {
		if fileExit {
			break
		}
		wg.Add(1)
		queueMaxSize <- true
		go func(fileName string) {
			defer func() {
				if fileErr != nil {
					fileExit = true
				}
				wg.Done()
				<-queueMaxSize
			}()
			object := prefix + fileName
			isSkipped := false
			localFileStat, err := os.Stat(localDir + fileName)
			if err != nil {
				fileErr = fmt.Errorf(" UploadFromDir Stat localFile: %s%s Error: %v", localDir, fileName, err)
				return
			}
			localFileSize := localFileStat.Size()
			localFileTime := localFileStat.ModTime()
			if options["replace"] != "true" {
				var objectHead, _ = c.Head(bucket, object)
				var objectHeadSize, _ = strconv.ParseInt(objectHead.Get("Content-Length"), 10, 64)
				if localFileSize == objectHeadSize {
					var objectTime, _ = time.Parse(c.dateTimeGMT, objectHead.Get("Last-Modified"))
					if objectTime.Unix() >= localFileTime.Unix() {
						isSkipped = true
						atomic.AddInt64(&tmpSkip, 1)
					}
				}
			}
			if !isSkipped {
				for i := 0; i < c.maxRetryNum; i++ {
					fd, oErr := os.Open(localDir + fileName)
					if oErr != nil {
						fileErr = fmt.Errorf(" UploadFromDir Open localFile: %s%s Error: %v", localDir, fileName, oErr)
						continue
					}
					bodySize := int(localFileSize)
					_, fileErr = c.Put(fd, bodySize, bucket, object, map[string]string{"disposition": fileName, "acl": options["acl"]})
					fd.Close()
					if fileErr != nil {
						continue
					}
					break
				}
				if fileErr != nil {
					return
				}
				atomic.AddInt64(&tmpSize, localFileSize)
				atomic.AddInt64(&tmpFinish, 1)
			}
			if percentChan != nil {
				percentChan <- total
			}
		}(fileList[fileNum])
	}
	wg.Wait()
	if fileErr != nil {
		return nil, fileErr
	}
	skip := int(atomic.LoadInt64(&tmpSkip))
	finish := int(atomic.LoadInt64(&tmpFinish))
	size := int(atomic.LoadInt64(&tmpSize))
	return map[string]int{"Total": total, "Skip": skip, "Finish": finish, "Size": size}, nil
}

// ListObject 查看列表
func (c *Client) ListObject(bucket string, options map[string]string) (*ListObjectResult, error) {
	param := ""
	if options["delimiter"] != "" {
		param += "&delimiter=" + options["delimiter"]
	}
	if options["marker"] != "" {
		param += "&marker=" + url.QueryEscape(options["marker"])
	}
	if options["max-keys"] != "" {
		param += "&max-keys=" + options["max-keys"]
	}
	if options["prefix"] != "" {
		param += "&prefix=" + url.QueryEscape(options["prefix"])
	}
	object := strings.TrimPrefix(param, "&")
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/?%s", host, object)
	method := "GET"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	headers["Authorization"] = c.sign(method, headers, "/", object)
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), body, nil)
	if err != nil {
		return nil, fmt.Errorf(" ListObject Bucket: %s Error: %v", bucket, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" ListObject Bucket: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", bucket, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	var listObject = &ListObjectResult{}
	if err = xml.Unmarshal(body.Bytes(), listObject); err != nil {
		return nil, fmt.Errorf(" ListObject Bucket: %s Error: %v", bucket, err)
	}
	return listObject, nil
}

// CopyAllObject 复制目录
func (c *Client) CopyAllObject(bucket, prefix, source string, options map[string]string, percentChan chan int) (map[string]int, error) {
	if prefix != "" {
		prefix = strings.TrimSuffix(prefix, "/") + "/"
	}
	maxKeys := "1000"
	if options["max-keys"] != "" {
		maxKeys = options["max-keys"]
	}
	marker := ""
	tmpSourceInfo := strings.Split(source, "/")
	sourceBucket := tmpSourceInfo[1]
	sourcePrefix := strings.Join(tmpSourceInfo[2:], "/")
	total := 0
	var threadNum = c.threadMaxNum
	if options["thread_num"] != "" {
		n, _ := strconv.Atoi(options["thread_num"])
		if n <= c.threadMaxNum && n >= c.threadMinNum {
			threadNum = n
		}
	}
	var queueMaxSize = make(chan bool, threadNum)
	defer close(queueMaxSize)
	var copyExit bool
	var tmpSize int64
	var tmpSkip int64
	var tmpFinish int64
	var wg sync.WaitGroup
LIST:
	sourceList, err := c.ListObject(sourceBucket, map[string]string{"prefix": sourcePrefix, "marker": marker, "max-keys": maxKeys})
	if err != nil {
		return nil, err
	}
	sourceObjectNum := len(sourceList.Contents)
	total += sourceObjectNum
	var fileErr error
	for fileNum := 0; fileNum < sourceObjectNum; fileNum++ {
		if copyExit {
			break
		}
		wg.Add(1)
		queueMaxSize <- true
		go func(objectInfo ListObjectContents) {
			defer func() {
				if fileErr != nil {
					copyExit = true
				}
				wg.Done()
				<-queueMaxSize
			}()
			//根据后缀过滤处理
			isSkipped := false
			if options["suffix"] != "" {
				suffixList := strings.Split(options["suffix"], ",")
				for _, tmpSuffix := range suffixList {
					if tmpSuffix != "" {
						if strings.HasSuffix(strings.ToLower(objectInfo.Key), tmpSuffix) {
							isSkipped = true
							break
						}
					}
				}
			}
			//支持自定义前缀
			object := prefix
			if options["full_path"] == "true" {
				object += strings.Replace(objectInfo.Key, sourcePrefix, "", -1)
			} else {
				object += path.Base(objectInfo.Key)
			}
			var sourceHead, _ = c.Head(sourceBucket, objectInfo.Key)
			var sourceHeadSize, _ = strconv.ParseInt(sourceHead.Get("Content-Length"), 10, 64)
			var disposition = internal.GetDisposition(sourceHead.Get("Content-Disposition"))
			if options["replace"] != "true" {
				var objectHead, _ = c.Head(bucket, object)
				var objectHeadSize, _ = strconv.ParseInt(objectHead.Get("Content-Length"), 10, 64)
				if sourceHeadSize == objectHeadSize {
					var objectTime, _ = time.Parse(c.dateTimeGMT, objectHead.Get("Last-Modified"))
					var sourceTime, _ = time.Parse(c.dateTimeGMT, sourceHead.Get("Last-Modified"))
					if objectTime.Unix() >= sourceTime.Unix() {
						isSkipped = true
					}
				}
			}
			if isSkipped {
				atomic.AddInt64(&tmpSkip, 1)
			} else {
				tmpSourceObject := "/" + sourceBucket + "/" + objectInfo.Key
				_, fileErr = c.CopyLargeFile(bucket, object, tmpSourceObject, map[string]string{"thread_num": options["thread_num"], "part_size": options["part_size"], "disposition": disposition, "acl": options["acl"]}, nil, nil)
				if fileErr != nil {
					return
				}
				atomic.AddInt64(&tmpSize, sourceHeadSize)
				atomic.AddInt64(&tmpFinish, 1)
			}
			if percentChan != nil {
				percentChan <- total
			}
		}(sourceList.Contents[fileNum])
	}
	wg.Wait()
	if fileErr == nil && sourceList.IsTruncated == "true" {
		marker = sourceList.Contents[sourceObjectNum-1].Key
		goto LIST
	}
	if fileErr != nil {
		return nil, fileErr
	}
	skip := int(atomic.LoadInt64(&tmpSkip))
	finish := int(atomic.LoadInt64(&tmpFinish))
	size := int(atomic.LoadInt64(&tmpSize))
	return map[string]int{"Total": total, "Skip": skip, "Finish": finish, "Size": size}, nil
}

// DeleteAllObject 删除目录
func (c *Client) DeleteAllObject(bucket, prefix string, options map[string]string, percentChan chan int) (map[string]int, error) {
	contents := make([]string, 0)
	counts := make([]int, 0)
	maxKeys := "1000"
	if options["max-keys"] != "" {
		maxKeys = options["max-keys"]
	}
	marker := ""
	total := 0
	var tmpFinish int64
LIST:
	list, err := c.ListObject(bucket, map[string]string{"prefix": prefix, "marker": marker, "max-keys": maxKeys})
	if err != nil {
		return nil, err
	}
	total += len(list.Contents)
	if total <= 0 {
		return map[string]int{"total": 0, "finish": 0}, nil
	}
	content := "<Delete>"
	content += "<Quiet>true</Quiet>"
	for _, v := range list.Contents {
		content += "<Object><Key>" + v.Key + "</Key></Object>"
		marker = v.Key
	}
	content += "</Delete>"
	contents = append(contents, content)
	counts = append(counts, len(list.Contents))
	if list.IsTruncated == "true" {
		goto LIST
	}

	var threadNum = c.threadMaxNum
	if options["thread_num"] != "" {
		n, _ := strconv.Atoi(options["thread_num"])
		if n <= c.threadMaxNum && n >= c.threadMinNum {
			threadNum = n
		}
	}
	var contentCount = len(contents)
	if contentCount < threadNum {
		threadNum = contentCount
	}
	var queueMaxSize = make(chan bool, threadNum)
	defer close(queueMaxSize)
	var fileErr error
	var fileExit bool
	var wg sync.WaitGroup
	for fileNum := 0; fileNum < contentCount; fileNum++ {
		if fileExit {
			break
		}
		wg.Add(1)
		queueMaxSize <- true
		go func(fileNum int, content string) {
			defer func() {
				if fileErr != nil {
					fileExit = true
				}
				wg.Done()
				<-queueMaxSize
			}()
			host := fmt.Sprintf("%s.%s", bucket, c.host)
			addr := fmt.Sprintf("http://%s/?delete", host)
			method := "POST"
			date := time.Now().UTC().Format(c.iso8601FormatDateTime)
			contentMd5 := internal.Base64Encode(internal.Md5Byte([]byte(content)))
			contentSha256 := hex.EncodeToString(hashSHA256([]byte(content)))
			headers := map[string]string{
				"host":                 host,
				"content-md5":          contentMd5,
				"x-amz-date":           date,
				"x-amz-content-sha256": contentSha256,
			}
			headers["Authorization"] = c.sign(method, headers, "/", "delete=")
			body := &bytes.Buffer{}
			header, cErr := internal.CURL(addr, method, headers, strings.NewReader(content), body, nil)
			if cErr != nil {
				fileErr = fmt.Errorf(" DeleteAllObject Prefix: %s Error: %v", prefix, cErr)
				return
			}
			var status = header.Get("StatusCode")
			var reqID = header.Get("X-Amz-Request-Id")
			if status != "200" {
				var errorMsg = &s3.Error{}
				_ = xml.Unmarshal(body.Bytes(), errorMsg)
				fileErr = fmt.Errorf(" DeleteAllObject Prefix: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", prefix, status, reqID, errorMsg.Code, errorMsg.Message)
				return
			}
			atomic.AddInt64(&tmpFinish, int64(counts[fileNum]))
			if percentChan != nil {
				percentChan <- total
			}
		}(fileNum, contents[fileNum])
	}
	wg.Wait()
	if fileErr != nil {
		return nil, fileErr
	}
	finish := int(atomic.LoadInt64(&tmpFinish))
	return map[string]int{"Total": total, "Finish": finish}, nil
}

// MoveAllObject 移动目录
func (c *Client) MoveAllObject(bucket, prefix, source string, options map[string]string, percentChan chan int) (map[string]int, error) {
	if prefix != "" {
		prefix = strings.TrimSuffix(prefix, "/") + "/"
	}
	tmpSourceInfo := strings.Split(source, "/")
	sourceBucket := tmpSourceInfo[1]
	sourcePrefix := strings.Join(tmpSourceInfo[2:], "/")
	if bucket == sourceBucket && prefix == sourcePrefix {
		return nil, fmt.Errorf("move soure-prefix and target-prefix same not allowed")
	}
	maxKeys := "1000"
	if options["max-keys"] != "" {
		maxKeys = options["max-keys"]
	}
	marker := ""
	total := 0
	var threadNum = c.threadMaxNum
	if options["thread_num"] != "" {
		n, _ := strconv.Atoi(options["thread_num"])
		if n <= c.threadMaxNum && n >= c.threadMinNum {
			threadNum = n
		}
	}
	var queueMaxSize = make(chan bool, threadNum)
	defer close(queueMaxSize)
	var copyExit bool
	var tmpSize int64
	var tmpSkip int64
	var tmpFinish int64
	var wg sync.WaitGroup
LIST:
	sourceList, err := c.ListObject(sourceBucket, map[string]string{"prefix": sourcePrefix, "marker": marker, "max-keys": maxKeys})
	if err != nil {
		return nil, err
	}
	sourceObjectNum := len(sourceList.Contents)
	total += sourceObjectNum
	var fileErr error
	for fileNum := 0; fileNum < sourceObjectNum; fileNum++ {
		if copyExit {
			break
		}
		wg.Add(1)
		queueMaxSize <- true
		go func(objectInfo ListObjectContents) {
			defer func() {
				if fileErr != nil {
					copyExit = true
				}
				wg.Done()
				<-queueMaxSize
			}()
			//根据后缀过滤处理
			isSkipped := false
			if options["suffix"] != "" {
				suffixList := strings.Split(options["suffix"], ",")
				for _, tmpSuffix := range suffixList {
					if tmpSuffix != "" {
						if strings.HasSuffix(strings.ToLower(objectInfo.Key), tmpSuffix) {
							isSkipped = true
							break
						}
					}
				}
			}
			//支持自定义前缀
			object := prefix
			if options["full_path"] == "true" {
				object += strings.Replace(objectInfo.Key, sourcePrefix, "", -1)
			} else {
				object += path.Base(objectInfo.Key)
			}
			var sourceHead, _ = c.Head(sourceBucket, objectInfo.Key)
			var sourceHeadSize, _ = strconv.ParseInt(sourceHead.Get("Content-Length"), 10, 64)
			var disposition = internal.GetDisposition(sourceHead.Get("Content-Disposition"))
			if options["replace"] != "true" {
				var objectHead, _ = c.Head(bucket, object)
				var objectHeadSize, _ = strconv.ParseInt(objectHead.Get("Content-Length"), 10, 64)
				if sourceHeadSize == objectHeadSize {
					var objectTime, _ = time.Parse(c.dateTimeGMT, objectHead.Get("Last-Modified"))
					var sourceTime, _ = time.Parse(c.dateTimeGMT, sourceHead.Get("Last-Modified"))
					if objectTime.Unix() >= sourceTime.Unix() {
						isSkipped = true
					}
				}
			}
			if isSkipped {
				atomic.AddInt64(&tmpSkip, 1)
			} else {
				tmpSourceObject := "/" + sourceBucket + "/" + objectInfo.Key
				_, fileErr = c.CopyLargeFile(bucket, object, tmpSourceObject, map[string]string{"thread_num": options["thread_num"], "part_size": options["part_size"], "disposition": disposition, "acl": options["acl"]}, nil, nil)
				if fileErr != nil {
					return
				}
				//删除源文件
				for i := 0; i < c.maxRetryNum; i++ {
					_, fileErr = c.Delete(sourceBucket, objectInfo.Key)
					if fileErr != nil {
						continue
					}
					break
				}
				if fileErr != nil {
					return
				}
				atomic.AddInt64(&tmpSize, sourceHeadSize)
				atomic.AddInt64(&tmpFinish, 1)
			}
			if percentChan != nil {
				percentChan <- total
			}
		}(sourceList.Contents[fileNum])
	}
	wg.Wait()
	if fileErr == nil && sourceList.IsTruncated == "true" {
		marker = sourceList.Contents[sourceObjectNum-1].Key
		goto LIST
	}
	if fileErr != nil {
		return nil, fileErr
	}
	skip := int(atomic.LoadInt64(&tmpSkip))
	finish := int(atomic.LoadInt64(&tmpFinish))
	size := int(atomic.LoadInt64(&tmpSize))
	return map[string]int{"Total": total, "Skip": skip, "Finish": finish, "Size": size}, nil
}

// DownloadAllObject 下载目录
func (c *Client) DownloadAllObject(bucket, prefix, localDir string, options map[string]string, percentChan chan int) (map[string]int, error) {
	maxKeys := "1000"
	if options["max-keys"] != "" {
		maxKeys = options["max-keys"]
	}
	marker := ""
	total := 0
	var threadNum = c.threadMaxNum
	if options["thread_num"] != "" {
		n, _ := strconv.Atoi(options["thread_num"])
		if n <= c.threadMaxNum && n >= c.threadMinNum {
			threadNum = n
		}
	}
	var queueMaxSize = make(chan bool, threadNum)
	defer close(queueMaxSize)
	var tmpSkip int64
	var tmpFinish int64
	var wg sync.WaitGroup
LIST:
	list, err := c.ListObject(bucket, map[string]string{"prefix": prefix, "marker": marker, "max-keys": maxKeys})
	if err != nil {
		return nil, err
	}
	objectNum := len(list.Contents)
	total += objectNum
	var fileErr error
	var fileExit bool
	for fileNum := 0; fileNum < objectNum; fileNum++ {
		if fileExit {
			break
		}
		wg.Add(1)
		queueMaxSize <- true
		go func(objectInfo ListObjectContents) {
			defer func() {
				if fileErr != nil {
					fileExit = true
				}
				wg.Done()
				<-queueMaxSize
			}()
			localFile := strings.TrimSuffix(localDir, "/") + "/" + objectInfo.Key
			isSkipped := false
			if options["replace"] != "true" {
				var objectHead, _ = c.Head(bucket, objectInfo.Key)
				var objectHeadSize, _ = strconv.ParseInt(objectHead.Get("Content-Length"), 10, 64)
				fileStat, sErr := os.Stat(localFile)
				if sErr == nil && objectHeadSize == fileStat.Size() {
					var objectTime, _ = time.Parse(c.dateTimeGMT, objectHead.Get("Last-Modified"))
					if objectTime.Unix() >= fileStat.ModTime().Unix() {
						isSkipped = true
						atomic.AddInt64(&tmpSkip, 1)
					}
				}
			}
			if !isSkipped {
				var getPercent = make(chan int)
				defer close(getPercent)
				go func() {
					for {
						_, ok := <-getPercent
						if !ok {
							break
						}
					}
				}()
				_, fileErr = c.Get(bucket, objectInfo.Key, localFile, map[string]string{
					"thread_num": options["thread_num"],
					"part_size":  options["part_size"],
				}, getPercent)
				if fileErr != nil {
					return
				}
				atomic.AddInt64(&tmpFinish, 1)
			}
			if percentChan != nil {
				percentChan <- total
			}
		}(list.Contents[fileNum])
	}
	wg.Wait()
	if fileErr == nil && list.IsTruncated == "true" {
		marker = list.Contents[objectNum-1].Key
		goto LIST
	}
	if fileErr != nil {
		return nil, fileErr
	}
	skip := int(atomic.LoadInt64(&tmpSkip))
	finish := int(atomic.LoadInt64(&tmpFinish))
	return map[string]int{"Total": total, "Skip": skip, "Finish": finish}, nil
}
