package s3

import (
	"io"
	"net/http"
)

// Client S3客户端接口
type Client interface {
	GetService() (*ServiceResult, error)
	CreateBucket(bucket string, options map[string]string) (http.Header, error)
	DeleteBucket(bucket string) (http.Header, error)
	ListPart(bucket string, options map[string]string) (*ListPartsResult, error)
	DeleteAllPart(bucket, prefix string, options map[string]string, percentChan chan int) (map[string]int, error)
	GetACL(bucket string) (*AclResult, error)
	SetACL(bucket string, options map[string]string) (http.Header, error)
	GetLifecycle(bucket string) (*LifecycleResult, error)
	SetLifecycle(bucket string, options map[string]string) (http.Header, error)
	DeleteLifecycle(bucket string) (http.Header, error)

	UploadLargeFile(filePath, bucket, object string, options map[string]string, percentChan chan int) (map[string]interface{}, error)
	MoveLargeFile(bucket, object, source string, options map[string]string) (map[string]interface{}, error)
	CopyLargeFile(bucket, object, source string, options map[string]string, percentChan chan int, exitChan <-chan bool) (map[string]interface{}, error)
	InitUpload(bucket, object string, options map[string]string) (*InitUploadResult, error)
	UploadPart(body io.Reader, bodySize int, bucket, object string, partNumber int, uploadID string) (http.Header, error)
	CancelPart(bucket, object string, uploadID string) (http.Header, error)
	CopyPart(partRange, bucket, object, source string, partNumber int, uploadID string, exitChan <-chan bool) (map[string]string, error)
	CompleteUpload(body []byte, bucket, object, uploadID string, objectSize int) (map[string]interface{}, error)

	SyncLargeFile(toClient Client, bucket, object, source string, options map[string]string, percentChan chan int) (map[string]interface{}, error)
	SyncAllObject(toClient Client, bucket, prefix, source string, options map[string]string, percentChan chan int) (map[string]int, error)

	UploadFile(filePath, bucket, object string, options map[string]string) (map[string]interface{}, error)
	Put(body io.Reader, bodySize int, bucket, object string, options map[string]string) (map[string]interface{}, error)
	Copy(bucket, object, source string, options map[string]string) (map[string]interface{}, error)
	Delete(bucket, object string) (http.Header, error)
	Head(bucket, object string) (http.Header, error)
	Get(bucket, object, localFile string, options map[string]string, percentChan chan int) (map[string]string, error)
	Cat(bucket, object, partRange string, dsc io.Writer) (http.Header, error)
	UploadFromDir(localDir, bucket, prefix string, options map[string]string, percentChan chan int) (map[string]int, error)
	ListObject(bucket string, options map[string]string) (*ListObjectResult, error)
	CopyAllObject(bucket, prefix, source string, options map[string]string, percentChan chan int) (map[string]int, error)
	DeleteAllObject(bucket, prefix string, options map[string]string, percentChan chan int) (map[string]int, error)
	MoveAllObject(bucket, prefix, source string, options map[string]string, percentChan chan int) (map[string]int, error)
	DownloadAllObject(bucket, prefix, localDir string, options map[string]string, percentChan chan int) (map[string]int, error)
}
