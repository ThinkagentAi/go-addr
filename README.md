# goaddr - 中国收货地址智能解析（Go 语言版）

## 功能概述

将一段非结构化的收货地址文本，智能解析为结构化的字段：

| 字段 | 说明 |
|------|------|
| `Name` | 收货人姓名 |
| `Mobile` | 手机号 / 座机号（支持虚拟号、国际区号、400/800 等） |
| `IdNumber` | 18 位身份证号码 |
| `PostCode` | 6 位邮政编码 |
| `Province` | 省 / 直辖市 / 自治区 / 特别行政区 |
| `City` | 市 / 自治州 / 地区 / 盟 |
| `Region` | 区 / 县 / 旗 |
| `Street` | 街道及以下详细地址 |
| `Address` | 省市区 + 街道的完整地址 |

核心特性：

- **统计特征匹配**：基于概率模型，解析成功率 98%+
- **高性能**：hash map 索引，单条解析约 0.06ms，1 秒可解析约 1.67 万条
- **别名识别**：自动处理省市区简称、自治州缩写、少数民族地名等
- **干扰排除**：路名/建筑名中包含省市名（如"北京东路""重庆大厦"）不会误判
- **格式兼容**：支持逗号、冒号、换行、空格等各种分隔符，以及 `+86`、`0086`、短横线分段等手机号写法

## 快速开始

### 环境要求

Go >= 1.11

### 安装

```bash
go get github.com/ThinkagentAi/goaddr
```

### 基本用法

```go
package main

import (
    "fmt"
    "github.com/ThinkagentAi/goaddr"
)

func main() {
    result := addr.Smart("张三 13800138000 龙华区龙华街道1980科技文化产业园3栋308 身份证120113196808214821")

    fmt.Println(result.Name)     // 张三
    fmt.Println(result.IdNumber) // 120113196808214821
    fmt.Println(result.Mobile)   // 13800138000
    fmt.Println(result.PostCode) // 570100
    fmt.Println(result.Province) // 广东省
    fmt.Println(result.City)     // 深圳市
    fmt.Println(result.Region)   // 龙华区
    fmt.Println(result.Street)   // 龙华街道1980科技文化产业园3栋308
    fmt.Println(result.Address)  // 深圳市龙华区龙华街道1980科技文化产业园3栋308
}
```

### 构建 HTTP 解析服务

使用 [fiber](https://github.com/gofiber/fiber) 几行代码即可搭建解析接口（gin 等框架类似）：

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/ThinkagentAi/goaddr"
)

func main() {
    app := fiber.New()

    app.Get("/", func(c *fiber.Ctx) error {
        result := addr.Smart(c.Query("addr"))
        return c.JSON(result)
    })

    app.Listen(":3000")
}
```

请求示例：

```
GET http://localhost:3000/?addr=张三 13800138000 龙华区龙华街道1980科技文化产业园3栋308
```

## API 说明

### `addr.Smart(str string) *Address`

一站式智能解析，自动完成信息分离和省市区提取。

### `addr.Decompose(info *Address, str string) *Address`

仅做基础信息分离（姓名、手机号、身份证、邮编、地址），不解析省市区。

### `addr.Parse(address *Address) *Address`

仅做省市区解析，需先通过 `Decompose` 设置 `Address` 字段。

## 索引数据更新

`areaMap/` 目录下的省市区索引由 `data/` 中的原始数据自动生成。如需更新行政区划数据：

```bash
# 修改 data/ 下的 province、city、region 数据文件
# 然后重新生成索引
cd generate
go run main.go
```

## 运行测试

```bash
go test -v ./...
```

`demo/main.go` 中包含 100+ 条覆盖各类场景的测试用例（直辖市、自治区、自治州、少数民族姓名、路名干扰、冷门地址等），可直接运行：

```bash
cd demo
go run main.go
```

## 致谢

- 本项目基于 PHP 版本 [pupuk/addr](https://github.com/pupuk/addr) 改写优化
- 感谢 [cxy-chenxuanyu](https://github.com/cxy-chenxuanyu) 在省市区解析部分的贡献

## License

[MIT](LICENSE)
