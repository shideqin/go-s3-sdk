package main

import (
	"fmt"
	"log"
	"os"
	"time"

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

	fmt.Println("=== 批量操作高级示例 ===")

	// 1. 批量上传文件
	fmt.Println("\n1. 批量上传测试文件...")
	testFiles := []struct {
		name    string
		content string
	}{
		{"test/batch/file1.txt", "这是测试文件1的内容"},
		{"test/batch/file2.txt", "这是测试文件2的内容"},
		{"test/batch/file3.txt", "这是测试文件3的内容"},
		{"test/batch/subdir/file4.txt", "这是子目录中文件4的内容"},
		{"test/batch/subdir/file5.txt", "这是子目录中文件5的内容"},
	}

	uploadedFiles := make([]string, 0)
	for i, file := range testFiles {
		fmt.Printf("上传文件 %d/%d: %s... ", i+1, len(testFiles), file.name)

		result, err := client.Put(
			[]byte(file.content),
			int64(len(file.content)),
			bucketName,
			file.name,
			map[string]interface{}{
				"Content-Type":        "text/plain; charset=utf-8",
				"x-amz-meta-batch-id": fmt.Sprintf("batch-%d", time.Now().Unix()),
			},
		)
		if err != nil {
			log.Printf("失败: %v", err)
		} else {
			fmt.Printf("成功 (ETag: %s)\n", result.ETag)
			uploadedFiles = append(uploadedFiles, file.name)
		}
	}

	fmt.Printf("成功上传 %d/%d 个文件\n", len(uploadedFiles), len(testFiles))

	// 2. 批量列出对象
	fmt.Println("\n2. 批量列出对象...")
	listResult, err := client.ListObject(bucketName, map[string]interface{}{
		"prefix":    "test/batch/",
		"max-keys":  "100",
		"delimiter": "",
	})
	if err != nil {
		log.Printf("列出对象失败: %v", err)
	} else {
		fmt.Printf("找到 %d 个对象:\n", len(listResult.Contents))
		totalSize := int64(0)
		for _, obj := range listResult.Contents {
			fmt.Printf("  - %s (大小: %d, 修改: %s)\n", obj.Key, obj.Size, obj.LastModified)
			totalSize += obj.Size
		}
		fmt.Printf("总大小: %d bytes\n", totalSize)
	}

	// 3. 批量复制对象
	fmt.Println("\n3. 批量复制对象...")
	copiedFiles := make([]string, 0)
	for i, fileName := range uploadedFiles {
		copyName := fmt.Sprintf("test/batch-copy/%s", fileName[len("test/batch/"):])
		fmt.Printf("复制文件 %d/%d: %s -> %s... ", i+1, len(uploadedFiles), fileName, copyName)

		result, err := client.Copy(
			bucketName,
			copyName,
			fmt.Sprintf("/%s/%s", bucketName, fileName),
			map[string]interface{}{
				"x-amz-meta-copy-source": fileName,
				"x-amz-meta-copy-time":   time.Now().Format(time.RFC3339),
			},
		)
		if err != nil {
			log.Printf("失败: %v", err)
		} else {
			fmt.Printf("成功 (ETag: %s)\n", result.ETag)
			copiedFiles = append(copiedFiles, copyName)
		}
	}

	fmt.Printf("成功复制 %d/%d 个文件\n", len(copiedFiles), len(uploadedFiles))

	// 4. 批量下载对象到本地目录
	fmt.Println("\n4. 批量下载对象...")
	downloadDir := "/tmp/batch-download"
	os.MkdirAll(downloadDir, 0755)

	downloadedFiles := make([]string, 0)
	for i, fileName := range uploadedFiles {
		localPath := fmt.Sprintf("%s/%s", downloadDir, fileName[len("test/batch/"):])

		// 创建本地目录
		localDir := localPath[:len(localPath)-len(fileName[len("test/batch/"):])-1]
		os.MkdirAll(localDir, 0755)

		fmt.Printf("下载文件 %d/%d: %s... ", i+1, len(uploadedFiles), fileName)

		result, err := client.Get(bucketName, fileName, localPath, nil, nil)
		if err != nil {
			log.Printf("失败: %v", err)
		} else {
			fmt.Printf("成功 (ETag: %s)\n", result.ETag)
			downloadedFiles = append(downloadedFiles, localPath)
		}
	}

	fmt.Printf("成功下载 %d/%d 个文件到 %s\n", len(downloadedFiles), len(uploadedFiles), downloadDir)

	// 5. 验证下载的文件
	fmt.Println("\n5. 验证下载的文件...")
	for i, localPath := range downloadedFiles {
		content, err := os.ReadFile(localPath)
		if err != nil {
			log.Printf("读取本地文件失败: %v", err)
			continue
		}

		expectedContent := testFiles[i].content
		if string(content) == expectedContent {
			fmt.Printf("✓ %s 内容验证成功\n", localPath)
		} else {
			fmt.Printf("✗ %s 内容验证失败\n", localPath)
		}
	}

	// 6. 批量移动对象（复制后删除原文件）
	fmt.Println("\n6. 批量移动对象...")
	movedFiles := make([]string, 0)
	for i, fileName := range uploadedFiles {
		moveName := fmt.Sprintf("test/batch-moved/%s", fileName[len("test/batch/"):])
		fmt.Printf("移动文件 %d/%d: %s -> %s... ", i+1, len(uploadedFiles), fileName, moveName)

		// 先复制
		result, err := client.Copy(
			bucketName,
			moveName,
			fmt.Sprintf("/%s/%s", bucketName, fileName),
			map[string]interface{}{
				"x-amz-meta-moved-from": fileName,
				"x-amz-meta-move-time":  time.Now().Format(time.RFC3339),
			},
		)
		if err != nil {
			log.Printf("复制失败: %v", err)
			continue
		}

		// 再删除原文件
		err = client.Delete(bucketName, fileName)
		if err != nil {
			log.Printf("删除原文件失败: %v", err)
			// 如果删除失败，也删除刚复制的文件
			client.Delete(bucketName, moveName)
			continue
		}

		fmt.Printf("成功 (ETag: %s)\n", result.ETag)
		movedFiles = append(movedFiles, moveName)
	}

	fmt.Printf("成功移动 %d/%d 个文件\n", len(movedFiles), len(uploadedFiles))

	// 7. 批量删除对象
	fmt.Println("\n7. 批量删除对象...")
	allFilesToDelete := append(copiedFiles, movedFiles...)

	deletedCount := 0
	for i, fileName := range allFilesToDelete {
		fmt.Printf("删除文件 %d/%d: %s... ", i+1, len(allFilesToDelete), fileName)

		err := client.Delete(bucketName, fileName)
		if err != nil {
			log.Printf("失败: %v", err)
		} else {
			fmt.Println("成功")
			deletedCount++
		}
	}

	fmt.Printf("成功删除 %d/%d 个文件\n", deletedCount, len(allFilesToDelete))

	// 8. 清理本地下载目录
	fmt.Println("\n8. 清理本地下载目录...")
	err = os.RemoveAll(downloadDir)
	if err != nil {
		log.Printf("清理本地目录失败: %v", err)
	} else {
		fmt.Printf("本地目录清理成功: %s\n", downloadDir)
	}

	// 9. 最终验证
	fmt.Println("\n9. 最终验证...")
	finalListResult, err := client.ListObject(bucketName, map[string]interface{}{
		"prefix":   "test/batch",
		"max-keys": "100",
	})
	if err != nil {
		log.Printf("最终验证失败: %v", err)
	} else {
		fmt.Printf("剩余对象数量: %d\n", len(finalListResult.Contents))
		if len(finalListResult.Contents) == 0 {
			fmt.Println("✓ 所有测试对象已清理完毕")
		} else {
			fmt.Println("剩余对象:")
			for _, obj := range finalListResult.Contents {
				fmt.Printf("  - %s\n", obj.Key)
			}
		}
	}

	fmt.Println("\n=== 批量操作示例完成 ===")
	fmt.Printf("演示了 %d 个文件的批量操作流程\n", len(testFiles))
}
