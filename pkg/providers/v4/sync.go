package v4

import (
	"fmt"
	"io"
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

// SyncLargeFile 分块同步文件
func (c *Client) SyncLargeFile(toClient s3.Client, bucket, object, source string, options map[string]string, percentChan chan int) (map[string]interface{}, error) {
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
	if !(objectSize > 0) {
		return nil, fmt.Errorf(" SyncLargeFile Object: %s Content-Length cant not zero", object)
	}
	var total = (objectSize + partSize - 1) / partSize
	if total < threadNum {
		threadNum = total
	}

	//初化化上传
	initUpload, initErr := toClient.InitUpload(bucket, object, map[string]string{"disposition": options["disposition"], "acl": options["acl"]})
	if initErr != nil {
		return nil, initErr
	}
	var syncPartList = make([]string, total)
	var queueMaxSize = make(chan bool, threadNum)
	defer close(queueMaxSize)
	var partErr error
	var partExit bool
	//sync分片
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
			//part范围,如：0-1023
			tmpStart := partNum * partSize
			tmpEnd := (partNum+1)*partSize - 1
			if tmpEnd > objectSize {
				tmpEnd = tmpStart + objectSize%partSize - 1
			}
			partRange := fmt.Sprintf("bytes=%d-%d", tmpStart, tmpEnd)
			tFile, tErr := os.CreateTemp("", fmt.Sprintf("aws-v4-sync-large%d", partNum))
			if tErr != nil {
				partErr = tErr
				return
			}
			defer func() {
				tFile.Close()
				os.Remove(tFile.Name())
			}()
			for i := 0; i < c.maxRetryNum; i++ {
				_, sErr := tFile.Seek(0, io.SeekStart)
				if sErr != nil {
					partErr = sErr
					continue
				}
				_, cErr := c.Cat(sourceBucket, sourceObject, partRange, tFile)
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
			stat, err := tFile.Stat()
			if err != nil {
				partErr = err
				return
			}
			for i := 0; i < c.maxRetryNum; i++ {
				tFile, partErr = os.Open(tFile.Name())
				if partErr != nil {
					continue
				}
				uploadPart, syncErr := toClient.UploadPart(tFile, int(stat.Size()), bucket, object, partNum+1, initUpload.UploadID)
				if syncErr != nil {
					partErr = syncErr
					tFile.Close()
					continue
				}
				partErr = nil
				syncPartList[partNum] = uploadPart.Get("Etag")
				break
			}
			if partErr != nil {
				return
			}
			if percentChan != nil {
				percentChan <- total
			}
		}(partNum)
		partNum++
	}
	wg.Wait()
	if partErr != nil {
		return nil, partErr
	}
	//sync完成
	completeSyncInfo := "<CompleteMultipartUpload>"
	for partNum, Etag := range syncPartList {
		completeSyncInfo += fmt.Sprintf("<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", partNum+1, Etag)
	}
	completeSyncInfo += "</CompleteMultipartUpload>"
	return toClient.CompleteUpload([]byte(completeSyncInfo), bucket, object, initUpload.UploadID, objectSize)
}

// SyncAllObject 同步目录
func (c *Client) SyncAllObject(toClient s3.Client, bucket, prefix, source string, options map[string]string, percentChan chan int) (map[string]int, error) {
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
	var tmpSize int64
	var tmpSkip int64
	var tmpFinish int64
	var fileErr error
	var fileExit bool
	var wg sync.WaitGroup
LIST:
	sourceList, listErr := c.ListObject(sourceBucket, map[string]string{"prefix": sourcePrefix, "marker": marker, "max-keys": maxKeys})
	if listErr != nil {
		return nil, listErr
	}
	sourceObjectNum := len(sourceList.Contents)
	total += sourceObjectNum
	for fileNum := 0; fileNum < sourceObjectNum; fileNum++ {
		if fileExit {
			break
		}
		wg.Add(1)
		queueMaxSize <- true
		go func(fileNum int, objectInfo ListObjectContents) {
			defer func() {
				if fileErr != nil {
					fileExit = true
				}
				wg.Done()
				<-queueMaxSize
			}()
			//支持自定义前缀
			object := prefix
			if options["full_path"] == "true" {
				object += strings.Replace(objectInfo.Key, sourcePrefix, "", -1)
			} else {
				object += path.Base(objectInfo.Key)
			}
			isSkipped := false
			var sourceHead, _ = c.Head(sourceBucket, objectInfo.Key)
			var disposition = internal.GetDisposition(sourceHead.Get("Content-Disposition"))
			var sourceHeadSize, _ = strconv.ParseInt(sourceHead.Get("Content-Length"), 10, 64)
			if options["replace"] != "true" {
				var objectHead, _ = c.Head(bucket, object)
				var objectHeadSize, _ = strconv.ParseInt(objectHead.Get("Content-Length"), 10, 64)
				if sourceHeadSize == objectHeadSize {
					var objectTime, _ = time.Parse(c.dateTimeGMT, objectHead.Get("Last-Modified"))
					var sourceTime, _ = time.Parse(c.dateTimeGMT, sourceHead.Get("Last-Modified"))
					if objectTime.Unix() >= sourceTime.Unix() {
						isSkipped = true
						atomic.AddInt64(&tmpSkip, 1)
					}
				}
			}

			if !isSkipped {
				tFile, tErr := os.CreateTemp("", fmt.Sprintf("aws-v4-sync-all%d", fileNum))
				if tErr != nil {
					fileErr = tErr
					return
				}
				defer func() {
					tFile.Close()
					os.Remove(tFile.Name())
				}()
				for i := 0; i < c.maxRetryNum; i++ {
					_, sErr := tFile.Seek(0, io.SeekStart)
					if sErr != nil {
						fileErr = sErr
						continue
					}
					_, cErr := c.Cat(sourceBucket, objectInfo.Key, "", tFile)
					if cErr != nil {
						fileErr = cErr
						continue
					}
					fileErr = nil
					break
				}
				if fileErr != nil {
					return
				}
				stat, err := tFile.Stat()
				if err != nil {
					fileErr = err
					return
				}
				for i := 0; i < c.maxRetryNum; i++ {
					tFile, fileErr = os.Open(tFile.Name())
					if fileErr != nil {
						continue
					}
					_, pErr := toClient.Put(tFile, int(stat.Size()), bucket, object, map[string]string{"disposition": disposition, "acl": options["acl"]})
					if pErr != nil {
						fileErr = pErr
						tFile.Close()
						continue
					}
					fileErr = nil
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
		}(fileNum, sourceList.Contents[fileNum])
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
