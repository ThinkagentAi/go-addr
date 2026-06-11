# goaddr — 中国收货地址智能解析

一个纯 Go 实现的收货地址智能解析库，能将一段非结构化的地址文本拆分为姓名、手机号、身份证号、邮编、省、市、区、街道等结构化字段。

基于统计特征与概率匹配，解析成功率 98%+；采用 hash map 索引，单条解析约 0.06ms。

## 解析字段

```go
type Address struct {
    Name     string `json:"name"`      // 收货人姓名
    Mobile   string `json:"mobile"`    // 手机号 / 座机号
    IdNumber string `json:"id_number"` // 18 位身份证号
    PostCode string `json:"post_code"` // 6 位邮编
    Province string `json:"province"`  // 省 / 直辖市 / 自治区 / 特别行政区
    City     string `json:"city"`      // 市 / 自治州 / 地区 / 盟
    Region   string `json:"region"`    // 区 / 县 / 旗 / 自治县
    Street   string `json:"street"`    // 街道及以下详细地址
    Address  string `json:"address"`   // 省市区 + 街道的完整地址
}
```

## 核心能力

| 能力 | 说明 |
|------|------|
| 信息分离 | 从混合文本中提取姓名、手机号、身份证号、邮编、地址 |
| 省市区解析 | 自动识别省/市/区，支持省略省市的反向推导 |
| 别名识别 | 直辖市简称（北京/上海）、自治区简称（内蒙古/新疆/西藏等）、自治州缩写（文山州 → 文山壮族苗族自治州）、城市俗称（呼市/乌市/锡盟/阿盟） |
| 同名消歧 | 多个同名区县（如朝阳区、龙华区）通过上下文交叉验证选择正确匹配 |
| 路名干扰排除 | 地址中的路名/建筑名含省市名（如"北京东路""重庆大厦""南京西路"）不会误判为行政区划 |
| 手机号兼容 | 支持 `+86`、`0086`、短横线分段（`136-3333-6666`）、空格分段（`138 0013 8000`）、虚拟号（`13800138000-1234`）、座机号、400/800 电话 |
| 分隔符兼容 | 空格、逗号、冒号、分号、换行等各种分隔方式均可解析 |
| 干扰词过滤 | 自动过滤"收货人""手机号""详细地址""身份证号"等标签词 |
| 直辖市处理 | 北京/上海/天津/重庆的市级自动填充为"XX市" |

## 安装

```bash
go get github.com/ThinkagentAi/goaddr
```

要求 Go >= 1.11。

## 快速开始

```go
package main

import (
    "fmt"
    "github.com/ThinkagentAi/goaddr"
)

func main() {
    result := goaddr.Smart("张三 13800138000 龙华区龙华街道1980科技文化产业园3栋308 身份证120113196808214821")

    fmt.Println(result.Name)     // 张三
    fmt.Println(result.Mobile)   // 13800138000
    fmt.Println(result.IdNumber) // 120113196808214821
    fmt.Println(result.Province) // 广东省
    fmt.Println(result.City)     // 深圳市
    fmt.Println(result.Region)   // 龙华区
    fmt.Println(result.Street)   // 龙华街道1980科技文化产业园3栋308
    fmt.Println(result.Address)  // 深圳市龙华区龙华街道1980科技文化产业园3栋308
}
```

## API

### `Smart(str string) *Address`

一站式智能解析。内部依次调用 `Decompose` 分离基础信息，再调用 `Parse` 解析省市区，最后提取街道地址。大多数场景只需调用此函数。

### `Decompose(info *Address, str string) *Address`

仅做基础信息分离：过滤干扰词 → 提取身份证号 → 提取手机号 → 提取邮编 → 按空格切分姓名与地址。不做省市区解析。

### `Parse(address *Address) *Address`

仅做省市区解析。需要先通过 `Decompose` 设置 `Address` 字段后调用。

## 解析示例

```
输入: "收货人：钱多多 手机号：13812345678 身份证号：320106199001011234 江苏省南京市玄武区中山路169号"
输出: Name=钱多多  Mobile=13812345678  IdNumber=320106199001011234  Province=江苏省  City=南京市  Region=玄武区  Street=中山路169号

输入: "赵六 18600002222 内蒙古呼市赛罕大学西街100号"
输出: Name=赵六  Mobile=18600002222  Province=内蒙古自治区  City=呼和浩特市  Region=赛罕区  Street=大学西街100号

输入: "李四 广东省深圳市南山区重庆南路科技园南路88号 13912345678"
输出: Name=李四  Mobile=13912345678  Province=广东省  City=深圳市  Region=南山区  Street=重庆南路科技园南路88号
```

## 构建 HTTP 解析服务

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/ThinkagentAi/goaddr"
)

func main() {
    app := fiber.New()

    app.Get("/", func(c *fiber.Ctx) error {
        result := goaddr.Smart(c.Query("addr"))
        return c.JSON(result)
    })

    app.Listen(":3000")
}
```

```
GET http://localhost:3000/?addr=张三 13800138000 龙华区龙华街道1980科技文化产业园3栋308
```

## 项目结构

```
goaddr/
├── addr.go              # 核心解析逻辑（Smart / Decompose / Parse）
├── addr_test.go         # 单元测试
├── areaMap/             # 省市区索引数据（由 go generate 自动生成）
│   ├── province.go      #   省级索引：ProvinceById / ProvinceByName / ProvinceByPid
│   ├── city.go          #   市级索引：CityById / CityByName / CityByPid
│   └── region.go        #   区县级索引：RegionById / RegionByName / RegionByPid
├── data/                # 行政区划原始数据
│   ├── province         #   省级数据
│   ├── city             #   市级数据
│   └── region           #   区县级数据
├── generate/            # 索引代码生成器
│   ├── main.go          #   go:generate 入口
│   └── autoCode/        #   自动生成逻辑
├── demo/                # 100+ 条场景测试用例
│   └── main.go
├── go.mod
└── LICENSE              # MIT
```

## 更新行政区划数据

`areaMap/` 下的索引文件由 `data/` 中的原始数据自动生成。如需更新行政区划：

```bash
# 1. 编辑 data/ 下的 province、city、region 数据文件
# 2. 重新生成索引
cd generate
go run main.go
```

## 测试

```bash
# 运行单元测试
go test -v ./...

# 运行 demo 场景测试（100+ 条用例，覆盖 17 类场景）
cd demo && go run main.go
```

demo 覆盖的测试场景：

1. **直辖市** — 北京、上海、天津、重庆
2. **普通省份** — 广东、江苏、浙江、四川等
3. **自治区** — 内蒙古、广西、西藏、宁夏、新疆
4. **自治州/地区/盟** — 延边、大理、阿克苏、锡林郭勒等
5. **含身份证号** — 18 位数字、末位 X
6. **含邮编** — 标注邮编、独立邮编、"邮政编码"前缀
7. **各种分隔符** — 逗号、冒号、换行、分号、混合分隔
8. **手机号变体** — 短横线、空格分段、`+86`/`0086` 前缀、虚拟号、座机号、400 电话、全号段（13/15/17/18/19）
9. **省略省/市** — 仅写区县+街道
10. **同名区县消歧** — 朝阳区（北京 vs 长春）、龙华区等
11. **纯地址** — 无姓名/手机
12. **路名干扰** — 北京东路、重庆大厦、南京西路等 50+ 条路名干扰用例
13. **少数民族姓名** — 维吾尔族、藏族、蒙古族、朝鲜族、壮族、彝族等 30+ 民族姓名
14. **冷门地址** — 神农架林区、三沙市、兵团城市、边境口岸等 50+ 条
15. **县级市/旗** — 义乌、昆山、准格尔旗等
16. **边界情况** — 空字符串、仅手机号、仅姓名

## 感谢

- 本项目基于 go 版本 [pupuk/addr](https://github.com/pupuk/addr) 改写优化


## License

[MIT](LICENSE)
