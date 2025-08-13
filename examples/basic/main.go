package main

import (
	"fmt"
	"log"
	"os"

	v2 "github.com/shideqin/go-s3-sdk/pkg/providers/v2"
	v4 "github.com/shideqin/go-s3-sdk/pkg/providers/v4"
	"github.com/shideqin/go-s3-sdk/pkg/s3"
)

func main() {
	// 从环境变量或使用默认值
	version := getEnvOrDefault("S3_VERSION", "v2")
	endpoint := getEnvOrDefault("S3_ENDPOINT", "oss-cn-hangzhou.aliyuncs.com")
	accessKeyID := getEnvOrDefault("S3_ACCESS_KEY_ID", "your-access-key-id")
	accessKeySecret := getEnvOrDefault("S3_SECRET_ACCESS_KEY", "your-secret-access-key")

	fmt.Printf("=== Go S3 SDK 基础示例 ===\n")
	fmt.Printf("协议版本: %s\n", version)
	fmt.Printf("端点: %s\n", endpoint)
	fmt.Printf("Access Key ID: %s\n", accessKeyID)

	// 创建客户端
	client := GetClient(version, endpoint, accessKeyID, accessKeySecret)
	if client == nil {
		log.Fatal("创建客户端失败")
	}

	fmt.Println("\n✓ 客户端创建成功")
	fmt.Printf("客户端类型: %T\n", client)

	// 尝试获取存储桶列表（如果配置了有效的凭证）
	if accessKeyID != "your-access-key-id" && accessKeySecret != "your-secret-access-key" {
		fmt.Println("\n尝试获取存储桶列表...")
		result, err := client.GetService()
		if err != nil {
			log.Printf("获取存储桶列表失败: %v", err)
		} else {
			fmt.Printf("成功获取到 %d 个存储桶\n", len(result.Buckets.Bucket))
			for _, bucket := range result.Buckets.Bucket {
				fmt.Printf("- %s (创建时间: %s)\n", bucket.Name, bucket.CreationDate)
			}
		}
	} else {
		fmt.Println("\n提示: 设置环境变量以测试实际功能:")
		fmt.Println("export S3_ACCESS_KEY_ID=\"your-real-access-key-id\"")
		fmt.Println("export S3_SECRET_ACCESS_KEY=\"your-real-secret-access-key\"")
		fmt.Println("export S3_ENDPOINT=\"your-s3-endpoint\"")
		fmt.Println("export S3_VERSION=\"v2\" # 或 v4")
	}

	fmt.Println("\n=== 基础示例完成 ===")
}

// GetClient 获得存储客户端
func GetClient(version, host, accessKeyID, accessKeySecret string) s3.Client {
	var client s3.Client
	if version == "v4" {
		client = v4.New(host, accessKeyID, accessKeySecret)
	} else {
		client = v2.New(host, accessKeyID, accessKeySecret)
	}
	return client
}

// getEnvOrDefault 获取环境变量值，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
