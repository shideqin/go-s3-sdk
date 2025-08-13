# Go S3 SDK

[![Go Version](https://img.shields.io/badge/Go-1.24.5+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

一个支持 **S3 v2/v4 标准协议**的 Go SDK，提供统一的接口来操作兼容 S3 协议的对象存储服务（如阿里云 OSS、腾讯云 COS、AWS S3 等）。

## ✨ 特性

- 🔧 **双协议支持**: 同时支持 S3 v2 和 v4 签名协议
- 🎯 **统一接口**: 提供一致的 API，无需关心底层协议差异
- 🚀 **高性能**: 支持大文件分块上传、并发操作
- 📁 **丰富功能**: 完整的存储桶和对象操作
- 🔄 **数据同步**: 支持跨存储服务的数据同步
- 📊 **进度监控**: 上传/下载进度实时反馈
- 🛡️ **错误处理**: 完善的错误处理和重试机制

## 🚀 快速开始

### 安装

```bash
go get github.com/shideqin/go-s3-sdk
```

### 基本使用

```go
package main

import (
    "fmt"
    "log"

    "github.com/shideqin/go-s3-sdk/pkg/providers/v2"
    "github.com/shideqin/go-s3-sdk/pkg/providers/v4"
    "github.com/shideqin/go-s3-sdk/pkg/s3"
)

func main() {
    // 创建 v2 客户端（适用于阿里云 OSS）
    client := v2.New("oss-cn-hangzhou.aliyuncs.com", "your-access-key-id", "your-access-key-secret")
    
    // 或创建 v4 客户端（适用于 AWS S3）
    // client := v4.New("s3.amazonaws.com", "your-access-key-id", "your-access-key-secret")
    
    // 获取存储桶列表
    result, err := client.GetService()
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("存储桶数量: %d\n", len(result.Buckets.Bucket))
}
```

## 📚 API 文档

### 客户端创建

#### V2 客户端（阿里云 OSS、腾讯云 COS）
```go
client := v2.New(host, accessKeyID, accessKeySecret)
```

#### V4 客户端（AWS S3、MinIO）
```go
client := v4.New(host, accessKeyID, accessKeySecret)
```

### 存储桶操作

| 方法 | 说明 | 参数 |
|------|------|------|
| `GetService()` | 获取存储桶列表 | - |
| `CreateBucket(bucket, options)` | 创建存储桶 | bucket: 桶名<br>options: 可选参数 |
| `DeleteBucket(bucket)` | 删除存储桶 | bucket: 桶名 |
| `GetACL(bucket)` | 获取访问控制列表 | bucket: 桶名 |
| `SetACL(bucket, options)` | 设置访问控制列表 | bucket: 桶名<br>options: ACL 配置 |
| `GetLifecycle(bucket)` | 获取生命周期规则 | bucket: 桶名 |
| `SetLifecycle(bucket, options)` | 设置生命周期规则 | bucket: 桶名<br>options: 规则配置 |
| `DeleteLifecycle(bucket)` | 删除生命周期规则 | bucket: 桶名 |

### 对象操作

#### 基础操作
| 方法 | 说明 | 参数 |
|------|------|------|
| `Put(body, bodySize, bucket, object, options)` | 上传对象 | body: 数据流<br>bodySize: 数据大小<br>bucket: 桶名<br>object: 对象名<br>options: 可选参数 |
| `Get(bucket, object, localFile, options, percentChan)` | 下载对象 | bucket: 桶名<br>object: 对象名<br>localFile: 本地文件路径<br>options: 可选参数<br>percentChan: 进度通道 |
| `Delete(bucket, object)` | 删除对象 | bucket: 桶名<br>object: 对象名 |
| `Head(bucket, object)` | 获取对象元数据 | bucket: 桶名<br>object: 对象名 |
| `Copy(bucket, object, source, options)` | 复制对象 | bucket: 目标桶<br>object: 目标对象<br>source: 源对象<br>options: 可选参数 |
| `ListObject(bucket, options)` | 列出对象 | bucket: 桶名<br>options: 过滤参数 |

#### 文件操作
| 方法 | 说明 | 参数 |
|------|------|------|
| `UploadFile(filePath, bucket, object, options)` | 上传文件 | filePath: 本地文件路径<br>bucket: 桶名<br>object: 对象名<br>options: 可选参数 |
| `UploadLargeFile(filePath, bucket, object, options, percentChan)` | 大文件分块上传 | filePath: 本地文件路径<br>bucket: 桶名<br>object: 对象名<br>options: 可选参数<br>percentChan: 进度通道 |
| `UploadFromDir(localDir, bucket, prefix, options, percentChan)` | 批量上传目录 | localDir: 本地目录<br>bucket: 桶名<br>prefix: 对象前缀<br>options: 可选参数<br>percentChan: 进度通道 |

#### 批量操作
| 方法 | 说明 | 参数 |
|------|------|------|
| `CopyAllObject(bucket, prefix, source, options, percentChan)` | 批量复制对象 | bucket: 桶名<br>prefix: 对象前缀<br>source: 源前缀<br>options: 可选参数<br>percentChan: 进度通道 |
| `DeleteAllObject(bucket, prefix, options, percentChan)` | 批量删除对象 | bucket: 桶名<br>prefix: 对象前缀<br>options: 可选参数<br>percentChan: 进度通道 |
| `MoveAllObject(bucket, prefix, source, options, percentChan)` | 批量移动对象 | bucket: 桶名<br>prefix: 目标前缀<br>source: 源前缀<br>options: 可选参数<br>percentChan: 进度通道 |
| `DownloadAllObject(bucket, prefix, localDir, options, percentChan)` | 批量下载对象 | bucket: 桶名<br>prefix: 对象前缀<br>localDir: 本地目录<br>options: 可选参数<br>percentChan: 进度通道 |

#### 分块上传
| 方法 | 说明 | 参数 |
|------|------|------|
| `InitUpload(bucket, object, options)` | 初始化分块上传 | bucket: 桶名<br>object: 对象名<br>options: 可选参数 |
| `UploadPart(body, bodySize, bucket, object, partNumber, uploadID)` | 上传分块 | body: 分块数据<br>bodySize: 分块大小<br>bucket: 桶名<br>object: 对象名<br>partNumber: 分块编号<br>uploadID: 上传ID |
| `CompleteUpload(body, bucket, object, uploadID, objectSize)` | 完成分块上传 | body: 完成请求体<br>bucket: 桶名<br>object: 对象名<br>uploadID: 上传ID<br>objectSize: 对象总大小 |
| `CancelPart(bucket, object, uploadID)` | 取消分块上传 | bucket: 桶名<br>object: 对象名<br>uploadID: 上传ID |

#### 数据同步
| 方法 | 说明 | 参数 |
|------|------|------|
| `SyncLargeFile(toClient, bucket, object, source, options, percentChan)` | 同步大文件 | toClient: 目标客户端<br>bucket: 桶名<br>object: 对象名<br>source: 源对象<br>options: 可选参数<br>percentChan: 进度通道 |
| `SyncAllObject(toClient, bucket, prefix, source, options, percentChan)` | 批量同步对象 | toClient: 目标客户端<br>bucket: 桶名<br>prefix: 对象前缀<br>source: 源前缀<br>options: 可选参数<br>percentChan: 进度通道 |

## 📖 使用示例

### 上传文件
```go
// 小文件直接上传
result, err := client.UploadFile("/path/to/local/file.txt", "my-bucket", "remote/file.txt", nil)
if err != nil {
    log.Fatal(err)
}

// 大文件分块上传（带进度监控）
percentChan := make(chan int, 1)
go func() {
    for percent := range percentChan {
        fmt.Printf("上传进度: %d%%\n", percent)
    }
}()

result, err := client.UploadLargeFile("/path/to/large/file.zip", "my-bucket", "large-file.zip", nil, percentChan)
if err != nil {
    log.Fatal(err)
}
```

### 下载文件
```go
// 下载文件（带进度监控）
percentChan := make(chan int, 1)
go func() {
    for percent := range percentChan {
        fmt.Printf("下载进度: %d%%\n", percent)
    }
}()

result, err := client.Get("my-bucket", "remote/file.txt", "/path/to/local/file.txt", nil, percentChan)
if err != nil {
    log.Fatal(err)
}
```

### 批量操作
```go
// 批量上传目录
percentChan := make(chan int, 1)
go func() {
    for percent := range percentChan {
        fmt.Printf("上传进度: %d%%\n", percent)
    }
}()

result, err := client.UploadFromDir("/local/directory", "my-bucket", "remote/prefix/", nil, percentChan)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("成功上传 %d 个文件\n", result["success"])
```

### 跨存储同步
```go
// 创建两个不同的客户端
sourceClient := v2.New("oss-cn-hangzhou.aliyuncs.com", "source-key", "source-secret")
targetClient := v4.New("s3.amazonaws.com", "target-key", "target-secret")

// 同步数据
percentChan := make(chan int, 1)
go func() {
    for percent := range percentChan {
        fmt.Printf("同步进度: %d%%\n", percent)
    }
}()

result, err := sourceClient.SyncAllObject(targetClient, "source-bucket", "data/", "backup/", nil, percentChan)
if err != nil {
    log.Fatal(err)
}
```

## 🔧 配置选项

### 客户端配置
- `partMaxSize`: 分块最大大小（默认：10MB）
- `partMinSize`: 分块最小大小（默认：1MB）
- `maxRetryNum`: 最大重试次数（默认：10）
- `threadMaxNum`: 最大并发线程数（默认：10）
- `threadMinNum`: 最小并发线程数（默认：1）

### 操作选项
- `Content-Type`: 内容类型
- `Cache-Control`: 缓存控制
- `Content-Encoding`: 内容编码
- `Content-Disposition`: 内容处置
- `x-oss-server-side-encryption`: 服务端加密
- 其他自定义头部

## 🌐 支持的存储服务

| 服务商 | 协议版本 | 端点示例 |
|--------|----------|----------|
| 阿里云 OSS | v2 | `oss-cn-hangzhou.aliyuncs.com` |
| 腾讯云 COS | v2/v4 | `cos.ap-guangzhou.myqcloud.com` |
| AWS S3 | v4 | `s3.amazonaws.com` |
| MinIO | v2/v4 | `localhost:9000` |
| 华为云 OBS | v2/v4 | `obs.cn-north-1.myhuaweicloud.com` |

## 📁 项目结构

```
go-s3-sdk/
├── pkg/
│   ├── providers/
│   │   ├── v2/          # S3 v2 协议实现
│   │   └── v4/          # S3 v4 协议实现
│   ├── s3/              # 统一接口定义
│   └── internal/        # 内部基础组件
├── examples/
│   └── basic/           # 基础示例
└── tests/               # 测试文件
```

## 📚 示例说明

### 基础示例 (examples/basic)
展示 SDK 的基本用法，包括客户端创建和简单的存储桶操作。

## 🧪 运行示例

```bash
# 克隆项目
git clone https://github.com/shideqin/go-s3-sdk.git
cd go-s3-sdk

# 运行基础示例
cd examples/basic
go mod tidy
go run main.go
```

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 🔗 相关链接

- [AWS S3 API 文档](https://docs.aws.amazon.com/s3/latest/API/)
- [阿里云 OSS API 文档](https://help.aliyun.com/product/31815.html)
- [腾讯云 COS API 文档](https://cloud.tencent.com/document/product/436)
