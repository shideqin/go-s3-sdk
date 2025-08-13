package v2

import (
	"crypto/hmac"
	"crypto/sha1"
	"sort"
	"strings"

	"github.com/shideqin/go-s3-sdk/pkg/internal"
)

func (c *Client) sign(method string, headers map[string]string, bucket, object string) string {
	var keyList []string
	LF := "\n"
	sign := method + LF
	for key := range headers {
		keyList = append(keyList, key)
	}
	sort.Strings(keyList)
	for _, key := range keyList {
		if strings.Contains(key, "x-amz-") {
			sign += key + ":" + headers[key] + LF
		} else {
			sign += headers[key] + LF
		}
	}
	sign += "/"
	if bucket != "" {
		sign += bucket
	}
	if object != "" {
		sign += "/" + object
	}
	return "AWS " + c.accessKeyID + ":" + internal.Base64Encode([]byte(hmacEncode(sign, c.accessKeySecret)))
}

func hmacEncode(sign, key string) string {
	h := hmac.New(sha1.New, []byte(key))
	_, _ = h.Write([]byte(sign))
	return string(h.Sum(nil))
}
