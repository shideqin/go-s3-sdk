package main

import (
	"fmt"
	"log"
	"os"

	v4 "github.com/shideqin/go-s3-sdk/pkg/providers/v4"
)

func main() {
	// 从环境变量获取配置
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKeyID := os.Getenv("S3_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("S3_SECRET_ACCESS_KEY")
	bucketName := os.Getenv("S3_BUCKET")

	if endpoint == "" || accessKeyID == "" || accessKeySecret == "" || bucketName == "" {
		log.Fatal("请设置环境变量: S3_ENDPOINT, S3_ACCESS_KEY_ID, S3_SECRET_ACCESS_KEY, S3_BUCKET")
	}

	// 创建客户端
	client := v4.New(endpoint, accessKeyID, accessKeySecret)

	fmt.Println("=== 分块上传高级示例 ===")

	// 创建一个大文件用于测试
	objectName := "test/large-file-multipart.dat"
	fmt.Printf("目标对象: %s\n", objectName)

	// 1. 初始化分块上传
	fmt.Println("\n1. 初始化分块上传...")
	initResult, err := client.InitUpload(bucketName, objectName, map[string]interface{}{
		"Content-Type":           "application/octet-stream",
		"x-amz-meta-upload-type": "multipart-demo",
	})
	if err != nil {
		log.Fatalf("初始化分块上传失败: %v", err)
	}
	fmt.Printf("UploadID: %s\n", initResult.UploadID)

	// 2. 准备分块数据
	partSize := 5 * 1024 * 1024 // 5MB per part
	totalParts := 3
	parts := make([]map[string]interface{}, totalParts)

	fmt.Printf("\n2. 上传 %d 个分块（每个 %d MB）...\n", totalParts, partSize/(1024*1024))

	for i := 0; i < totalParts; i++ {
		partNumber := i + 1

		// 创建测试数据
		partData := make([]byte, partSize)
		for j := 0; j < partSize; j++ {
			partData[j] = byte((i*partSize + j) % 256)
		}

		fmt.Printf("上传分块 %d/%d... ", partNumber, totalParts)

		// 上传分块
		partResult, err := client.UploadPart(
			partData,
			int64(partSize),
			bucketName,
			objectName,
			partNumber,
			initResult.UploadID,
		)
		if err != nil {
			log.Printf("上传分块 %d 失败: %v", partNumber, err)
			// 取消分块上传
			client.CancelPart(bucketName, objectName, initResult.UploadID)
			return
		}

		parts[i] = map[string]interface{}{
			"PartNumber": partNumber,
			"ETag":       partResult.ETag,
		}

		fmt.Printf("成功 (ETag: %s)\n", partResult.ETag)
	}

	// 3. 完成分块上传
	fmt.Println("\n3. 完成分块上传...")

	// 构建完成请求的 XML
	completeXML := "<CompleteMultipartUpload>"
	for _, part := range parts {
		completeXML += fmt.Sprintf(
			"<Part><PartNumber>%v</PartNumber><ETag>%v</ETag></Part>",
			part["PartNumber"], part["ETag"],
		)
	}
	completeXML += "</CompleteMultipartUpload>"

	completeResult, err := client.CompleteUpload(
		[]byte(completeXML),
		bucketName,
		objectName,
		initResult.UploadID,
		int64(totalParts*partSize),
	)
	if err != nil {
		log.Printf("完成分块上传失败: %v", err)
		// 取消分块上传
		client.CancelPart(bucketName, objectName, initResult.UploadID)
		return
	}

	fmt.Printf("分块上传完成！\n")
	fmt.Printf("最终 ETag: %s\n", completeResult.ETag)

	// 4. 验证上传的文件
	fmt.Println("\n4. 验证上传的文件...")
	headResult, err := client.Head(bucketName, objectName)
	if err != nil {
		log.Printf("获取对象信息失败: %v", err)
	} else {
		fmt.Printf("文件大小: %d bytes\n", headResult.ContentLength)
		fmt.Printf("内容类型: %s\n", headResult.ContentType)
		fmt.Printf("ETag: %s\n", headResult.ETag)

		expectedSize := int64(totalParts * partSize)
		if headResult.ContentLength == expectedSize {
			fmt.Println("✓ 文件大小验证成功")
		} else {
			fmt.Printf("✗ 文件大小不匹配，期望: %d，实际: %d\n", expectedSize, headResult.ContentLength)
		}
	}

	// 5. 清理测试文件
	fmt.Println("\n5. 清理测试文件...")
	err = client.Delete(bucketName, objectName)
	if err != nil {
		log.Printf("删除文件失败: %v", err)
	} else {
		fmt.Println("测试文件删除成功")
	}

	fmt.Println("\n=== 分块上传示例完成 ===")
	fmt.Printf("总共上传了 %d MB 的数据\n", (totalParts*partSize)/(1024*1024))
}
