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
	"time"

	"github.com/shideqin/go-s3-sdk/pkg/internal"
	"github.com/shideqin/go-s3-sdk/pkg/s3"
)

// InitUploadResult 初始化上传结果
type InitUploadResult = s3.InitUploadResult

// CopyPartResult 复制分片结果
type CopyPartResult = s3.CopyPartResult

// CompleteUploadResult 完成上传结果
type CompleteUploadResult = s3.CompleteUploadResult

// UploadLargeFile 分块上传文件
func (c *Client) UploadLargeFile(filePath, bucket, object string, options map[string]string, percentChan chan int) (map[string]interface{}, error) {
	//open本地文件
	fd, openErr := os.Open(filePath)
	if openErr != nil {
		return nil, fmt.Errorf(" UploadLargeFile Open localFile: %s Error: %v", filePath, openErr)
	}
	defer fd.Close()
	var partSize = c.partMaxSize
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
	if object == "" {
		object = path.Base(filePath)
	}
	if strings.TrimSuffix(object, "/") == path.Dir(object) {
		object = path.Dir(object) + "/" + path.Base(filePath)
	}
	fileStat, _ := fd.Stat()
	fileSize := int(fileStat.Size())
	var total = (fileSize + partSize - 1) / partSize
	if total < threadNum {
		threadNum = total
	}
	//初化化上传
	initUpload, initErr := c.InitUpload(bucket, object, map[string]string{"disposition": options["disposition"], "acl": options["acl"]})
	if initErr != nil {
		return nil, initErr
	}
	var uploadPartList = make([]string, total)
	var queueMaxSize = make(chan bool, threadNum)
	defer close(queueMaxSize)
	var uploadExit bool
	var partErr error
	var wg sync.WaitGroup
	for partNum := 0; partNum < total; partNum++ {
		if uploadExit {
			break
		}
		wg.Add(1)
		queueMaxSize <- true
		go func(partNum int, fd *os.File) {
			defer func() {
				if partErr != nil {
					uploadExit = true
				}
				wg.Done()
				<-queueMaxSize
			}()
			offset := partNum * partSize
			num := partSize
			if fileSize-offset < num {
				num = fileSize - offset
			}
			for i := 0; i < c.maxRetryNum; i++ {
				partReader := io.NewSectionReader(fd, int64(offset), int64(num))
				partReaderSize := int(partReader.Size())
				uploadPart, upErr := c.UploadPart(partReader, partReaderSize, bucket, object, partNum+1, initUpload.UploadID)
				if upErr != nil {
					partErr = upErr
					continue
				}
				partErr = nil
				uploadPartList[partNum] = uploadPart.Get("Etag")
				break
			}
			if partErr != nil {
				return
			}
			//进度条
			if percentChan != nil {
				percentChan <- total
			}
		}(partNum, fd)
	}
	wg.Wait()
	if partErr != nil {
		return nil, partErr
	}
	//上传完成
	completeUploadInfo := "<CompleteMultipartUpload>"
	for partNum, Etag := range uploadPartList {
		completeUploadInfo += fmt.Sprintf("<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", partNum+1, Etag)
	}
	completeUploadInfo += "</CompleteMultipartUpload>"
	return c.CompleteUpload([]byte(completeUploadInfo), bucket, object, initUpload.UploadID, fileSize)
}

// CopyLargeFile 分块复制文件
func (c *Client) CopyLargeFile(bucket, object, source string, options map[string]string, percentChan chan int, exitChan <-chan bool) (map[string]interface{}, error) {
	tmpSourceInfo := strings.Split(source, "/")
	sourceBucket := tmpSourceInfo[1]
	sourceObject := strings.Join(tmpSourceInfo[2:], "/")
	sourceHead, headErr := c.Head(sourceBucket, sourceObject)
	if headErr != nil {
		return nil, headErr
	}
	var partSize = c.partMaxSize
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

	if object == "" {
		object = path.Base(sourceObject)
	}
	if strings.TrimSuffix(object, "/") == path.Dir(object) {
		object = path.Dir(object) + "/" + path.Base(sourceObject)
	}
	var objectSize, _ = strconv.Atoi(sourceHead.Get("Content-Length"))
	var total = (objectSize + partSize - 1) / partSize
	if total < threadNum {
		threadNum = total
	}

	//初化化上传
	initUpload, initErr := c.InitUpload(bucket, object, map[string]string{"disposition": options["disposition"], "acl": options["acl"]})
	if initErr != nil {
		return nil, initErr
	}
	var copyPartList = make([]string, total)

	//取消处理
	var copyCancel bool
	var copyExitChan = make(chan bool)
	defer close(copyExitChan)
	if exitChan != nil {
		go func() {
			for {
				exit, ok := <-exitChan
				if !ok {
					break
				}
				if exit {
					copyCancel = true
					copyExitChan <- true
					break
				}
			}
		}()
	}

	//copy分片
	var queueMaxSize = make(chan bool, threadNum)
	defer close(queueMaxSize)
	var partErr error
	var copyExit bool
	var wg sync.WaitGroup
	for partNum := 0; partNum < total; partNum++ {
		if copyExit {
			break
		}
		wg.Add(1)
		queueMaxSize <- true
		go func(partNum int) {
			defer func() {
				if partErr != nil {
					copyExit = true
				}
				wg.Done()
				<-queueMaxSize
			}()
			//part范围,如：0-1023
			tmpStart := partNum * partSize
			tmpEnd := (partNum+1)*partSize - 1
			if tmpEnd > objectSize {
				tmpEnd = tmpStart + objectSize%partSize - 1
			}
			partRange := fmt.Sprintf("bytes=%d-%d", tmpStart, tmpEnd)
			for i := 0; i < c.maxRetryNum; i++ {
				if copyCancel {
					partErr = fmt.Errorf("canceled")
					break
				}
				copyPart, copyErr := c.CopyPart(partRange, bucket, object, source, partNum+1, initUpload.UploadID, copyExitChan)
				if copyErr != nil {
					partErr = copyErr
					continue
				}
				partErr = nil
				copyPartList[partNum] = copyPart["Etag"]
				break
			}
			if partErr != nil {
				return
			}
			//进度条
			if percentChan != nil {
				percentChan <- total
			}
		}(partNum)
	}
	wg.Wait()
	if partErr != nil {
		return nil, partErr
	}
	//copy完成
	completeCopyInfo := "<CompleteMultipartUpload>"
	for partNum, Etag := range copyPartList {
		completeCopyInfo += fmt.Sprintf("<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", partNum+1, Etag)
	}
	completeCopyInfo += "</CompleteMultipartUpload>"
	return c.CompleteUpload([]byte(completeCopyInfo), bucket, object, initUpload.UploadID, objectSize)
}

// MoveLargeFile 移动文件
func (c *Client) MoveLargeFile(bucket, object, source string, options map[string]string) (map[string]interface{}, error) {
	tmpSourceInfo := strings.Split(source, "/")
	sourceBucket := tmpSourceInfo[1]
	sourceObject := strings.Join(tmpSourceInfo[2:], "/")
	if bucket == sourceBucket && object == sourceObject {
		return nil, fmt.Errorf("move soure-object and target-object same not allowed")
	}
	sourceHead, hErr := c.Head(sourceBucket, sourceObject)
	if hErr != nil {
		return nil, hErr
	}
	var disposition = internal.GetDisposition(sourceHead.Get("Content-Disposition"))
	copied, cErr := c.CopyLargeFile(bucket, object, source, map[string]string{"thread_num": options["thread_num"], "part_size": options["part_size"], "disposition": disposition, "acl": options["acl"]}, nil, nil)
	if cErr != nil {
		return nil, cErr
	}
	//删除源文件
	for i := 0; i < c.maxRetryNum; i++ {
		_, cErr = c.Delete(sourceBucket, sourceObject)
		if cErr != nil {
			continue
		}
		break
	}
	if cErr != nil {
		return nil, cErr
	}
	return copied, nil
}

func (c *Client) InitUpload(bucket, object string, options map[string]string) (*InitUploadResult, error) {
	nObject := url.QueryEscape(object)
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/%s?uploads", host, nObject)
	method := "POST"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	if options["acl"] != "" {
		headers["x-amz-acl"] = options["acl"]
	}
	headers["Authorization"] = c.sign(method, headers, "/"+nObject, "uploads=")
	if options["disposition"] != "" {
		headers["Content-Disposition"] = fmt.Sprintf(`attachment; filename="%s"`, options["disposition"])
	}
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), body, nil)
	if err != nil {
		return nil, fmt.Errorf(" InitUpload Object: %s Error: %v", object, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" InitUpload Object: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", object, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	var initUpload = &InitUploadResult{}
	if err = xml.Unmarshal(body.Bytes(), initUpload); err != nil {
		return nil, fmt.Errorf(" InitUpload Object: %s Error: %v", object, err)
	}
	return initUpload, nil
}

func (c *Client) UploadPart(content io.Reader, bodySize int, bucket, object string, partNumber int, uploadID string) (http.Header, error) {
	nObject := url.QueryEscape(object)
	subObject := fmt.Sprintf("partNumber=%d&uploadId=%s", partNumber, uploadID)
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/%s?%s", host, nObject, subObject)
	method := "PUT"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	contentSha256 := hex.EncodeToString(hashSHA256Reader(content))
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": contentSha256,
	}
	headers["Authorization"] = c.sign(method, headers, "/"+nObject, subObject)
	headers["Content-Length"] = fmt.Sprintf("%d", bodySize)
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, content, body, nil)
	if err != nil {
		return nil, fmt.Errorf(" UploadPart Object: %s Error: %v", object, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" UploadPart Object: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", object, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	return header, nil
}

func (c *Client) CancelPart(bucket, object string, uploadID string) (http.Header, error) {
	nObject := url.QueryEscape(object)
	subObject := fmt.Sprintf("uploadId=%s", uploadID)
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/%s?%s", host, nObject, subObject)
	method := "DELETE"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	headers["Authorization"] = c.sign(method, headers, "/"+nObject, subObject)
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), nil, nil)
	if err != nil {
		return nil, fmt.Errorf(" CancelPart Object: %s Error: %v", object, err)
	}
	return header, nil
}

func (c *Client) CopyPart(partRange, bucket, object, source string, partNumber int, uploadID string, copyExitChan <-chan bool) (map[string]string, error) {
	nObject := url.QueryEscape(object)
	subObject := fmt.Sprintf("partNumber=%d&uploadId=%s", partNumber, uploadID)
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/%s?%s", host, nObject, subObject)
	method := "PUT"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                    host,
		"x-amz-date":              date,
		"x-amz-content-sha256":    c.emptyStringSHA256,
		"x-amz-copy-source":       source,
		"x-amz-copy-source-range": partRange,
	}
	headers["Authorization"] = c.sign(method, headers, "/"+nObject, subObject)
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), body, copyExitChan)
	if err != nil {
		return nil, fmt.Errorf(" CopyPart Object: %s Error: %v", object, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" CopyPart Object: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", object, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	var copyPart = &CopyPartResult{}
	if err := xml.Unmarshal(body.Bytes(), copyPart); err != nil {
		return nil, fmt.Errorf(" CopyPart Object: %s Error: %v", object, err)
	}
	return map[string]string{"Etag": copyPart.ETag}, nil
}

func (c *Client) CompleteUpload(content []byte, bucket, object, uploadID string, objectSize int) (map[string]interface{}, error) {
	nObject := url.QueryEscape(object)
	subObject := fmt.Sprintf("uploadId=%s", uploadID)
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/%s?%s", host, nObject, subObject)
	method := "POST"
	contentSha256 := hex.EncodeToString(hashSHA256(content))
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": contentSha256,
	}
	headers["Authorization"] = c.sign(method, headers, "/"+nObject, subObject)
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, bytes.NewReader(content), body, nil)
	if err != nil {
		return nil, fmt.Errorf(" CompleteUpload Object: %s Error: %v", object, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" CompleteUpload Object: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", object, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	var completeUpload = &CompleteUploadResult{}
	if err = xml.Unmarshal(body.Bytes(), completeUpload); err != nil {
		return nil, fmt.Errorf(" CompleteUpload Object: %s Error: %v", object, err)
	}
	return map[string]interface{}{
		"Location": fmt.Sprintf("http://%s.%s/%s", bucket, c.host, object),
		"Bucket":   completeUpload.Bucket,
		"Key":      completeUpload.Key,
		"ETag":     completeUpload.ETag,
		"Size":     objectSize,
	}, nil
}
