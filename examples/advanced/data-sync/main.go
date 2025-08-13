package main

import (
	"fmt"
	"log"
	"os"
	"time"

	v2 "github.com/shideqin/go-s3-sdk/pkg/providers/v2"
	v4 "github.com/shideqin/go-s3-sdk/pkg/providers/v4"
	"github.com/shideqin/go-s3-sdk/pkg/s3"
)

func main() {
	fmt.Println("=== 数据同步高级示例 ===")

	// 配置源存储（例如：阿里云 OSS）
	sourceEndpoint := os.Getenv("SOURCE_ENDPOINT")
	sourceAccessKeyID := os.Getenv("SOURCE_ACCESS_KEY_ID")
	sourceAccessKeySecret := os.Getenv("SOURCE_ACCESS_KEY_SECRET")
	sourceBucket := os.Getenv("SOURCE_BUCKET")

	// 配置目标存储（例如：AWS S3 或 MinIO）
	targetEndpoint := os.Getenv("TARGET_ENDPOINT")
	targetAccessKeyID := os.Getenv("TARGET_ACCESS_KEY_ID")
	targetAccessKeySecret := os.Getenv("TARGET_ACCESS_KEY_SECRET")
	targetBucket := os.Getenv("TARGET_BUCKET")

	if sourceEndpoint == "" || sourceAccessKeyID == "" || sourceAccessKeySecret == "" || sourceBucket == "" {
		log.Fatal("请设置源存储环境变量: SOURCE_ENDPOINT, SOURCE_ACCESS_KEY_ID, SOURCE_ACCESS_KEY_SECRET, SOURCE_BUCKET")
	}

	if targetEndpoint == "" || targetAccessKeyID == "" || targetAccessKeySecret == "" || targetBucket == "" {
		log.Fatal("请设置目标存储环境变量: TARGET_ENDPOINT, TARGET_ACCESS_KEY_ID, TARGET_ACCESS_KEY_SECRET, TARGET_BUCKET")
	}

	// 创建源客户端（使用 v2 协议，适用于阿里云 OSS）
	var sourceClient s3.Client
	sourceProtocol := os.Getenv("SOURCE_PROTOCOL")
	if sourceProtocol == "v4" {
		sourceClient = v4.New(sourceEndpoint, sourceAccessKeyID, sourceAccessKeySecret)
		fmt.Printf("源存储: %s (v4 协议)\n", sourceEndpoint)
	} else {
		sourceClient = v2.New(sourceEndpoint, sourceAccessKeyID, sourceAccessKeySecret)
		fmt.Printf("源存储: %s (v2 协议)\n", sourceEndpoint)
	}

	// 创建目标客户端（使用 v4 协议，适用于 AWS S3/MinIO）
	var targetClient s3.Client
	targetProtocol := os.Getenv("TARGET_PROTOCOL")
	if targetProtocol == "v2" {
		targetClient = v2.New(targetEndpoint, targetAccessKeyID, targetAccessKeySecret)
		fmt.Printf("目标存储: %s (v2 协议)\n", targetEndpoint)
	} else {
		targetClient = v4.New(targetEndpoint, targetAccessKeyID, targetAccessKeySecret)
		fmt.Printf("目标存储: %s (v4 协议)\n", targetEndpoint)
	}

	// 1. 在源存储创建测试数据
	fmt.Println("\n1. 在源存储创建测试数据...")
	testData := []struct {
		key     string
		content string
		size    string
	}{
		{"sync-test/small-file1.txt", "这是小文件1的内容", "小文件"},
		{"sync-test/small-file2.txt", "这是小文件2的内容", "小文件"},
		{"sync-test/dir1/file3.txt", "这是目录1中文件3的内容", "小文件"},
		{"sync-test/dir2/file4.txt", "这是目录2中文件4的内容", "小文件"},
	}

	// 创建一个大文件用于测试大文件同步
	largeContent := make([]byte, 2*1024*1024) // 2MB
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	testData = append(testData, struct {
		key     string
		content string
		size    string
	}{"sync-test/large-file.dat", string(largeContent), "大文件"})

	uploadedFiles := make([]string, 0)
	for i, data := range testData {
		fmt.Printf("上传 %s (%s) %d/%d... ", data.key, data.size, i+1, len(testData))

		result, err := sourceClient.Put(
			[]byte(data.content),
			int64(len(data.content)),
			sourceBucket,
			data.key,
			map[string]interface{}{
				"Content-Type":         "text/plain",
				"x-amz-meta-sync-test": "true",
				"x-amz-meta-created":   time.Now().Format(time.RFC3339),
			},
		)
		if err != nil {
			log.Printf("失败: %v", err)
		} else {
			fmt.Printf("成功 (ETag: %s)\n", result.ETag)
			uploadedFiles = append(uploadedFiles, data.key)
		}
	}

	fmt.Printf("在源存储创建了 %d 个测试文件\n", len(uploadedFiles))

	// 2. 列出源存储中的对象
	fmt.Println("\n2. 列出源存储中的对象...")
	sourceList, err := sourceClient.ListObject(sourceBucket, map[string]interface{}{
		"prefix":   "sync-test/",
		"max-keys": "100",
	})
	if err != nil {
		log.Printf("列出源对象失败: %v", err)
		return
	}

	fmt.Printf("源存储中找到 %d 个对象:\n", len(sourceList.Contents))
	sourceTotalSize := int64(0)
	for _, obj := range sourceList.Contents {
		fmt.Printf("  - %s (大小: %d)\n", obj.Key, obj.Size)
		sourceTotalSize += obj.Size
	}
	fmt.Printf("源存储总大小: %d bytes\n", sourceTotalSize)

	// 3. 同步小文件
	fmt.Println("\n3. 同步小文件...")
	smallFiles := make([]string, 0)
	for _, obj := range sourceList.Contents {
		if obj.Size < 1024*1024 { // 小于 1MB 的文件
			smallFiles = append(smallFiles, obj.Key)
		}
	}

	syncedSmallFiles := 0
	for i, fileName := range smallFiles {
		fmt.Printf("同步小文件 %d/%d: %s... ", i+1, len(smallFiles), fileName)

		// 从源存储下载
		tempFile := fmt.Sprintf("/tmp/sync-%d.tmp", i)
		_, err := sourceClient.Get(sourceBucket, fileName, tempFile, nil, nil)
		if err != nil {
			log.Printf("下载失败: %v", err)
			continue
		}

		// 读取文件内容
		content, err := os.ReadFile(tempFile)
		if err != nil {
			log.Printf("读取临时文件失败: %v", err)
			os.Remove(tempFile)
			continue
		}

		// 上传到目标存储
		_, err = targetClient.Put(
			content,
			int64(len(content)),
			targetBucket,
			fileName,
			map[string]interface{}{
				"Content-Type":           "text/plain",
				"x-amz-meta-synced-from": fmt.Sprintf("%s/%s", sourceBucket, fileName),
				"x-amz-meta-sync-time":   time.Now().Format(time.RFC3339),
			},
		)
		if err != nil {
			log.Printf("上传失败: %v", err)
		} else {
			fmt.Println("成功")
			syncedSmallFiles++
		}

		// 清理临时文件
		os.Remove(tempFile)
	}

	fmt.Printf("成功同步 %d/%d 个小文件\n", syncedSmallFiles, len(smallFiles))

	// 4. 同步大文件（使用分块上传）
	fmt.Println("\n4. 同步大文件...")
	largeFiles := make([]string, 0)
	for _, obj := range sourceList.Contents {
		if obj.Size >= 1024*1024 { // 大于等于 1MB 的文件
			largeFiles = append(largeFiles, obj.Key)
		}
	}

	syncedLargeFiles := 0
	for i, fileName := range largeFiles {
		fmt.Printf("同步大文件 %d/%d: %s... ", i+1, len(largeFiles), fileName)

		// 使用 SDK 的大文件同步功能
		result, err := sourceClient.SyncLargeFile(
			targetClient,
			targetBucket,
			fileName,
			fmt.Sprintf("%s/%s", sourceBucket, fileName),
			map[string]interface{}{
				"x-amz-meta-synced-from": fmt.Sprintf("%s/%s", sourceBucket, fileName),
				"x-amz-meta-sync-time":   time.Now().Format(time.RFC3339),
			},
			nil, // 进度通道，这里不使用
		)
		if err != nil {
			log.Printf("失败: %v", err)
		} else {
			fmt.Printf("成功 (ETag: %s)\n", result["ETag"])
			syncedLargeFiles++
		}
	}

	fmt.Printf("成功同步 %d/%d 个大文件\n", syncedLargeFiles, len(largeFiles))

	// 5. 验证同步结果
	fmt.Println("\n5. 验证同步结果...")
	targetList, err := targetClient.ListObject(targetBucket, map[string]interface{}{
		"prefix":   "sync-test/",
		"max-keys": "100",
	})
	if err != nil {
		log.Printf("列出目标对象失败: %v", err)
	} else {
		fmt.Printf("目标存储中找到 %d 个对象:\n", len(targetList.Contents))
		targetTotalSize := int64(0)
		for _, obj := range targetList.Contents {
			fmt.Printf("  - %s (大小: %d)\n", obj.Key, obj.Size)
			targetTotalSize += obj.Size
		}
		fmt.Printf("目标存储总大小: %d bytes\n", targetTotalSize)

		// 比较大小
		if sourceTotalSize == targetTotalSize {
			fmt.Println("✓ 同步验证成功：源存储和目标存储大小一致")
		} else {
			fmt.Printf("✗ 同步验证失败：源存储 %d bytes，目标存储 %d bytes\n", sourceTotalSize, targetTotalSize)
		}
	}

	// 6. 批量同步演示（使用 SDK 的批量同步功能）
	fmt.Println("\n6. 批量同步演示...")
	batchResult, err := sourceClient.SyncAllObject(
		targetClient,
		targetBucket,
		"sync-test-batch/", // 目标前缀
		fmt.Sprintf("%s/sync-test/", sourceBucket), // 源前缀
		map[string]interface{}{
			"x-amz-meta-batch-sync": "true",
			"x-amz-meta-batch-time": time.Now().Format(time.RFC3339),
		},
		nil, // 进度通道
	)
	if err != nil {
		log.Printf("批量同步失败: %v", err)
	} else {
		fmt.Printf("批量同步成功: %v\n", batchResult)
	}

	// 7. 清理测试数据
	fmt.Println("\n7. 清理测试数据...")

	// 清理源存储
	fmt.Println("清理源存储...")
	for i, fileName := range uploadedFiles {
		fmt.Printf("删除源文件 %d/%d: %s... ", i+1, len(uploadedFiles), fileName)
		err := sourceClient.Delete(sourceBucket, fileName)
		if err != nil {
			log.Printf("失败: %v", err)
		} else {
			fmt.Println("成功")
		}
	}

	// 清理目标存储
	fmt.Println("清理目标存储...")
	if targetList != nil {
		for i, obj := range targetList.Contents {
			fmt.Printf("删除目标文件 %d/%d: %s... ", i+1, len(targetList.Contents), obj.Key)
			err := targetClient.Delete(targetBucket, obj.Key)
			if err != nil {
				log.Printf("失败: %v", err)
			} else {
				fmt.Println("成功")
			}
		}
	}

	// 清理批量同步的文件
	fmt.Println("清理批量同步文件...")
	batchList, err := targetClient.ListObject(targetBucket, map[string]interface{}{
		"prefix":   "sync-test-batch/",
		"max-keys": "100",
	})
	if err == nil {
		for i, obj := range batchList.Contents {
			fmt.Printf("删除批量同步文件 %d/%d: %s... ", i+1, len(batchList.Contents), obj.Key)
			err := targetClient.Delete(targetBucket, obj.Key)
			if err != nil {
				log.Printf("失败: %v", err)
			} else {
				fmt.Println("成功")
			}
		}
	}

	fmt.Println("\n=== 数据同步示例完成 ===")
	fmt.Printf("演示了从 %s 到 %s 的数据同步\n", sourceEndpoint, targetEndpoint)
	fmt.Printf("总共同步了 %d 个文件，大小 %d bytes\n", syncedSmallFiles+syncedLargeFiles, sourceTotalSize)
}
