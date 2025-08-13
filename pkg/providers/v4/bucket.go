package v4

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shideqin/go-s3-sdk/pkg/internal"
	"github.com/shideqin/go-s3-sdk/pkg/s3"
)

// ServiceResult 获取bucket列表结果
type ServiceResult = s3.ServiceResult

// AclResult 获取bucket Acl列表结果
type AclResult = s3.AclResult

// LifecycleResult 获取bucket Lifecycle列表结果
type LifecycleResult = s3.LifecycleResult

// ListPartsResult 获取分块列表结果
type ListPartsResult = s3.ListPartsResult

// GetService 获取bucket列表
func (c *Client) GetService() (*ServiceResult, error) {
	addr := fmt.Sprintf("http://%s/", c.host)
	method := "GET"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 c.host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	headers["Authorization"] = c.sign(method, headers, "/", "")
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), body, nil)
	if err != nil {
		return nil, fmt.Errorf(" GetService Error: %v", err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" GetService StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", status, reqID, errorMsg.Code, errorMsg.Message)
	}
	var service = &ServiceResult{}
	if err = xml.Unmarshal(body.Bytes(), service); err != nil {
		return nil, fmt.Errorf(" GetService Error: %v", err)
	}
	return service, nil
}

// CreateBucket 创建bucket
func (c *Client) CreateBucket(bucket string, options map[string]string) (http.Header, error) {
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/", host)
	method := "PUT"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	region := strings.Split(c.host, ".")[1]
	content := `<CreateBucketConfiguration><LocationConstraint>` + region + `</LocationConstraint></CreateBucketConfiguration>`
	contentSha256 := hex.EncodeToString(hashSHA256([]byte(content)))
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": contentSha256,
	}
	if options["acl"] != "" {
		headers["x-amz-acl"] = options["acl"]
	}
	headers["Authorization"] = c.sign(method, headers, "/", "")
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, strings.NewReader(content), body, nil)
	if err != nil {
		return nil, fmt.Errorf(" CreateBucket Bucket: %s Error: %v", bucket, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" CreateBucket Bucket: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", bucket, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	return header, nil
}

// DeleteBucket 删除bucket
func (c *Client) DeleteBucket(bucket string) (http.Header, error) {
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/", host)
	method := "DELETE"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	headers["Authorization"] = c.sign(method, headers, "/", "")
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), body, nil)
	if err != nil {
		return nil, fmt.Errorf(" DeleteBucket Bucket: %s Error: %v", bucket, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" DeleteBucket Bucket: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", bucket, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	return header, nil
}

// ListPart 查看分块列表
func (c *Client) ListPart(bucket string, options map[string]string) (*ListPartsResult, error) {
	param := ""
	if options["delimiter"] != "" {
		param += "&delimiter=" + options["delimiter"]
	}
	if options["key-marker"] != "" {
		param += "&key-marker=" + options["key-marker"]
	}
	if options["max-keys"] != "" {
		param += "&max-uploads=" + options["max-keys"]
	}
	if options["prefix"] != "" {
		param += "&prefix=" + options["prefix"]
	}
	object := strings.TrimPrefix(param, "&")
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/?uploads&%s", host, object)
	method := "GET"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	headers["Authorization"] = c.sign(method, headers, "/", object+"&uploads=")
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), body, nil)
	if err != nil {
		return nil, fmt.Errorf(" ListPart Bucket: %s Error: %v", bucket, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" ListPart Bucket: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", bucket, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	var ListParts = &ListPartsResult{}
	if err = xml.Unmarshal(body.Bytes(), ListParts); err != nil {
		return nil, fmt.Errorf(" ListPart Bucket: %s Error: %v", bucket, err)
	}
	return ListParts, nil
}

// DeleteAllPart 删除所有分块
func (c *Client) DeleteAllPart(bucket, prefix string, options map[string]string, percentChan chan int) (map[string]int, error) {
	contents := make([]map[string]string, 0)
	maxKeys := "1000"
	if options["max-keys"] != "" {
		maxKeys = options["max-keys"]
	}
	marker := ""
	total := 0
	var tmpFinish int64
	var tmpSkip int64
	var wg sync.WaitGroup
LIST:
	list, err := c.ListPart(bucket, map[string]string{"prefix": prefix, "key-marker": marker, "max-keys": maxKeys})
	if err != nil {
		return nil, err
	}
	total += len(list.Upload)
	if total <= 0 {
		return map[string]int{"Total": 0, "Finish": 0}, nil
	}
	expired, _ := strconv.Atoi(options["expired"])
	for _, v := range list.Upload {
		lastModified, err := time.Parse("2006-01-02T15:04:05.000Z", v.Initiated)
		if err == nil && time.Since(lastModified).Seconds() < float64(expired) {
			atomic.AddInt64(&tmpSkip, 1)
			continue
		}
		contents = append(contents, map[string]string{"Bucket": bucket, "Key": v.Key, "UploadID": v.UploadID})
	}
	if list.IsTruncated == "true" {
		marker = list.NextKeyMarker
		goto LIST
	}

	var threadNum = c.threadMaxNum
	if options["thread_num"] != "" {
		n, _ := strconv.Atoi(options["thread_num"])
		if n <= c.threadMaxNum && n >= c.threadMinNum {
			threadNum = n
		}
	}
	var contentSize = len(contents)
	if contentSize < threadNum {
		threadNum = contentSize
	}
	var queueMaxSize = make(chan bool, threadNum)
	defer close(queueMaxSize)
	var partErr error
	var partExit bool
	for partNum := 0; partNum < contentSize; partNum++ {
		if partExit {
			break
		}
		wg.Add(1)
		queueMaxSize <- true
		go func(partNum int, body map[string]string) {
			defer func() {
				if partErr != nil {
					partExit = true
				}
				wg.Done()
				<-queueMaxSize
			}()
			for i := 0; i < c.maxRetryNum; i++ {
				_, partErr = c.CancelPart(body["Bucket"], body["Key"], body["UploadID"])
				if partErr != nil {
					continue
				}
				partErr = nil
				break
			}
			if partErr != nil {
				return
			}
			atomic.AddInt64(&tmpFinish, 1)
			if percentChan != nil {
				percentChan <- total
			}
		}(partNum, contents[partNum])
	}
	wg.Wait()
	if partErr != nil {
		return nil, partErr
	}
	finish := int(atomic.LoadInt64(&tmpFinish))
	skip := int(atomic.LoadInt64(&tmpSkip))
	return map[string]int{"Total": total, "Finish": finish, "Skip": skip}, nil
}

// GetACL 获取bucket acl
func (c *Client) GetACL(bucket string) (*AclResult, error) {
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/?acl", host)
	method := "GET"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	headers["Authorization"] = c.sign(method, headers, "/", "acl=")
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), body, nil)
	if err != nil {
		return nil, fmt.Errorf(" GetACL Bucket: %s Error: %v", bucket, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" GetACL Bucket: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", bucket, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	var acl = &AclResult{}
	if err = xml.Unmarshal(body.Bytes(), acl); err != nil {
		return nil, fmt.Errorf(" GetACL Bucket: %s Error: %v", bucket, err)
	}
	return acl, nil
}

// SetACL 设置bucket acl
func (c *Client) SetACL(bucket string, options map[string]string) (http.Header, error) {
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/?acl", host)
	method := "PUT"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	if options["acl"] != "" {
		headers["x-amz-acl"] = options["acl"]
	}
	headers["Authorization"] = c.sign(method, headers, "/", "acl=")
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), body, nil)
	if err != nil {
		return nil, fmt.Errorf(" SetACL Bucket: %s Error: %v", bucket, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" SetACL Bucket: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", bucket, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	return header, nil
}

// GetLifecycle 获取bucket lifecycle
func (c *Client) GetLifecycle(bucket string) (*LifecycleResult, error) {
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/?lifecycle", host)
	method := "GET"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	headers["Authorization"] = c.sign(method, headers, "/", "lifecycle=")
	body := &bytes.Buffer{}
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), body, nil)
	if err != nil {
		return nil, fmt.Errorf(" GetLifecycle Bucket: %s Error: %v", bucket, err)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" GetLifecycle Bucket: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", bucket, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	var lifecycle = &LifecycleResult{}
	if err = xml.Unmarshal(body.Bytes(), lifecycle); err != nil {
		return nil, fmt.Errorf(" GetLifecycle Bucket: %s Error: %v", bucket, err)
	}
	return lifecycle, nil
}

// SetLifecycle 设置bucket lifecycle
func (c *Client) SetLifecycle(bucket string, options map[string]string) (http.Header, error) {
	content := "<LifecycleConfiguration>"
	lifecycle, lErr := c.GetLifecycle(bucket)
	if lErr == nil {
		for _, v := range lifecycle.Rules {
			content += "<Rule>"
			content += fmt.Sprintf("<ID>%s</ID>", v.ID)
			content += "<Status>Enabled</Status>"
			content += fmt.Sprintf("<Filter><Prefix>%s</Prefix></Filter>", v.Filter.Prefix)
			content += fmt.Sprintf("<Expiration><Days>%d</Days></Expiration>", v.Expiration.Days)
			content += "</Rule>"
		}
	}
	content += "<Rule>"
	content += fmt.Sprintf("<ID>%s</ID>", internal.UUID())
	content += "<Status>Enabled</Status>"
	content += fmt.Sprintf("<Filter><Prefix>%s</Prefix></Filter>", options["prefix"])
	content += fmt.Sprintf("<Expiration><Days>%s</Days></Expiration>", options["expiration"])
	content += "</Rule></LifecycleConfiguration>"

	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/?lifecycle", host)
	method := "PUT"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	contentMd5 := internal.Base64Encode(internal.Md5Byte([]byte(content)))
	contentSha256 := hex.EncodeToString(hashSHA256([]byte(content)))
	headers := map[string]string{
		"host":                 host,
		"content-md5":          contentMd5,
		"x-amz-date":           date,
		"x-amz-content-sha256": contentSha256,
	}
	headers["Authorization"] = c.sign(method, headers, "/", "lifecycle=")
	body := &bytes.Buffer{}
	header, cErr := internal.CURL(addr, method, headers, strings.NewReader(content), body, nil)
	if cErr != nil {
		return nil, fmt.Errorf(" SetLifecycle Bucket: %s Error: %v", bucket, cErr)
	}
	var status = header.Get("StatusCode")
	var reqID = header.Get("X-Amz-Request-Id")
	if status != "200" {
		var errorMsg = &s3.Error{}
		_ = xml.Unmarshal(body.Bytes(), errorMsg)
		return nil, fmt.Errorf(" SetLifecycle Bucket: %s StatusCode: %s X-Amz-Request-Id: %s Code: %s Message: %s", bucket, status, reqID, errorMsg.Code, errorMsg.Message)
	}
	return header, nil
}

// DeleteLifecycle 删除bucket lifecycle
func (c *Client) DeleteLifecycle(bucket string) (http.Header, error) {
	host := fmt.Sprintf("%s.%s", bucket, c.host)
	addr := fmt.Sprintf("http://%s/?lifecycle", host)
	method := "DELETE"
	date := time.Now().UTC().Format(c.iso8601FormatDateTime)
	headers := map[string]string{
		"host":                 host,
		"x-amz-date":           date,
		"x-amz-content-sha256": c.emptyStringSHA256,
	}
	headers["Authorization"] = c.sign(method, headers, "/", "lifecycle=")
	header, err := internal.CURL(addr, method, headers, strings.NewReader(""), nil, nil)
	if err != nil {
		return nil, fmt.Errorf(" DeleteLifecycle Bucket: %s Error: %v", bucket, err)
	}
	return header, nil
}
