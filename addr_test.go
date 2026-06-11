package addr

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSmart(t *testing.T) {
	data, err := os.ReadFile("data/test")
	if err != nil {
		fmt.Println("读取test文件失败，请检查文件！---", err)
		return
	}
	s := strings.Split(string(data), "\n")

	startT := time.Now() //计算当前时间
	for _, v := range s {
		marshal, err := json.Marshal(Smart(v))
		if err != nil {
			return
		}
		fmt.Println(string(marshal))
	}
	tc := time.Since(startT) //计算解析耗时
	fmt.Printf("time cost = %v\n", tc)
}

func TestSmartAdministrativeAliases(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Address
	}{
		{
			name: "省市区后缀省略",
			raw:  "张三 13800138000 广东深圳南山科技园南路88号",
			want: Address{Province: "广东省", City: "深圳市", Region: "南山区", Street: "科技园南路88号"},
		},
		{
			name: "同名区县带城市消歧",
			raw:  "李四 13800138000 广东省深圳市龙华区龙华街道1980科技文化产业园3栋308",
			want: Address{Province: "广东省", City: "深圳市", Region: "龙华区", Street: "龙华街道1980科技文化产业园3栋308"},
		},
		{
			name: "道路名包含其他城市不误判",
			raw:  "李四 广东省深圳市南山区重庆南路科技园南路88号 13912345678",
			want: Address{Province: "广东省", City: "深圳市", Region: "南山区", Street: "重庆南路科技园南路88号"},
		},
		{
			name: "直辖市完整写法",
			raw:  "王五 18600001111 北京市海淀区中关村大街1号",
			want: Address{Province: "北京", City: "北京市", Region: "海淀区", Street: "中关村大街1号"},
		},
		{
			name: "自治区和城市简称",
			raw:  "赵六 18600002222 内蒙古呼市赛罕大学西街100号",
			want: Address{Province: "内蒙古自治区", City: "呼和浩特市", Region: "赛罕区", Street: "大学西街100号"},
		},
		{
			name: "自治州简称",
			raw:  "钱七 18600003333 云南文山州砚山县工业园区",
			want: Address{Province: "云南省", City: "文山壮族苗族自治州", Region: "砚山县", Street: "工业园区"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Smart(tt.raw)
			gotParts := Address{
				Province: got.Province,
				City:     got.City,
				Region:   got.Region,
				Street:   got.Street,
			}
			if !reflect.DeepEqual(gotParts, tt.want) {
				t.Fatalf("Smart() location = %+v, want %+v", gotParts, tt.want)
			}
		})
	}
}

func TestSmartContactWithChinaCountryCode(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantName   string
		wantMobile string
	}{
		{
			name:       "plus86",
			raw:        "王五 +8618600001111 北京市海淀区中关村大街1号",
			wantName:   "王五",
			wantMobile: "18600001111",
		},
		{
			name:       "plus86WithSeparator",
			raw:        "王五 +86 18600001111 北京市海淀区中关村大街1号",
			wantName:   "王五",
			wantMobile: "18600001111",
		},
		{
			name:       "doubleZero86",
			raw:        "王五 008618600001111 北京市海淀区中关村大街1号",
			wantName:   "王五",
			wantMobile: "18600001111",
		},
		{
			name:       "plus85",
			raw:        "李四 广东省深圳市南山区科技园南路88号 +8513912345678",
			wantName:   "李四",
			wantMobile: "+8513912345678",
		},
		{
			name:       "doubleZeroInternational",
			raw:        "李四 广东省深圳市南山区科技园南路88号 008513912345678",
			wantName:   "李四",
			wantMobile: "008513912345678",
		},
		{
			name:       "domesticHyphenSeparated",
			raw:        "李明 136-3333-6666 四川省成都市锦江区春熙路108号",
			wantName:   "李明",
			wantMobile: "13633336666",
		},
		{
			name:       "domesticSpaceSeparated",
			raw:        "陈明 138 0013 8000 福建省福州市鼓楼区五四路100号",
			wantName:   "陈明",
			wantMobile: "13800138000",
		},
		{
			name:       "service400",
			raw:        "客服 400-123-4567 北京市朝阳区建国路100号",
			wantName:   "客服",
			wantMobile: "400-123-4567",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Smart(tt.raw)
			if got.Name != tt.wantName {
				t.Fatalf("Name = %q, want %q; parsed = %+v", got.Name, tt.wantName, got)
			}
			if got.Mobile != tt.wantMobile {
				t.Fatalf("Mobile = %q, want %q; parsed = %+v", got.Mobile, tt.wantMobile, got)
			}
		})
	}
}

func TestSmartBeijingChaoyangNotLiaoningChaoyang(t *testing.T) {
	got := Smart("客服 400-123-4567 北京市朝阳区建国路100号")

	if got.Province != "北京" || got.City != "北京市" || got.Region != "朝阳区" {
		t.Fatalf("location parsed = %+v, want 北京/北京市/朝阳区", got)
	}
	if got.Street != "建国路100号" {
		t.Fatalf("Street = %q, want %q; parsed = %+v", got.Street, "建国路100号", got)
	}
	if got.Address != "北京市朝阳区建国路100号" {
		t.Fatalf("Address = %q, want %q; parsed = %+v", got.Address, "北京市朝阳区建国路100号", got)
	}
}

func TestSmartRoadNameShouldNotOverrideLeadingProvince(t *testing.T) {
	got := Smart("迪丽热巴·迪力木拉提 新疆乌鲁木齐市新市区北京南路50号 13800001234")

	if got.Name != "迪丽热巴·迪力木拉提" {
		t.Fatalf("Name = %q, want %q; parsed = %+v", got.Name, "迪丽热巴·迪力木拉提", got)
	}
	if got.Province != "新疆维吾尔自治区" || got.City != "乌鲁木齐市" || got.Region != "新市区" {
		t.Fatalf("location parsed = %+v, want 新疆维吾尔自治区/乌鲁木齐市/新市区", got)
	}
	if got.Street != "北京南路50号" {
		t.Fatalf("Street = %q, want %q; parsed = %+v", got.Street, "北京南路50号", got)
	}
}

func TestSmartNameWithLabeledFields(t *testing.T) {
	raw := "收货人：钱多多 手机号：13812345678 身份证号：320106199001011234 江苏省南京市玄武区中山路169号"
	got := Smart(raw)

	if got.Name != "钱多多" {
		t.Fatalf("Name = %q, want %q; parsed = %+v", got.Name, "钱多多", got)
	}
	if got.Mobile != "13812345678" {
		t.Fatalf("Mobile = %q, want %q; parsed = %+v", got.Mobile, "13812345678", got)
	}
	if got.IdNumber != "320106199001011234" {
		t.Fatalf("IdNumber = %q, want %q; parsed = %+v", got.IdNumber, "320106199001011234", got)
	}
	if got.Province != "江苏省" || got.City != "南京市" || got.Region != "玄武区" {
		t.Fatalf("location parsed = %+v", got)
	}
}
