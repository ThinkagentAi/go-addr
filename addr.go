package goaddr

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ThinkagentAi/goaddr/areaMap"
)

type Address struct {
	IdNumber string `json:"id_number"`
	Mobile   string `json:"mobile"`
	PostCode string `json:"post_code"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Province string `json:"province"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Street   string `json:"street"`
}

// FilterWord 需要过滤掉收货地址中的常用说明字符，排除干扰词
// 按长度降序排列，确保长词优先匹配（避免 "详细地址" 中的 "地址" 被先替换）
var FilterWord = []string{
	"身份证号码", "手机号码", "电话号码", "所在地区", "详细地址", "收货人", "收件人",
	"身份证号", "手机号", "电话号", "联系人", "身份证", "号码", "收货", "邮编", "电话", "手机", "地址",
	"：", ":", "；", ";", "，", ",", "。", ".",
}

// provinceAlias 省级别名映射，处理直辖市等简称/全称不一致的情况
var provinceAlias = map[string]string{
	"北京市":  "北京",
	"天津市":  "天津",
	"上海市":  "上海",
	"重庆市":  "重庆",
	"北京市市": "北京",
	"天津市市": "天津",
	"上海市市": "上海",
	"重庆市市": "重庆",
	"内蒙古":  "内蒙古自治区",
	"广西":   "广西壮族自治区",
	"西藏":   "西藏自治区",
	"宁夏":   "宁夏回族自治区",
	"新疆":   "新疆维吾尔自治区",
	"香港":   "香港特别行政区",
	"澳门":   "澳门特别行政区",
}

// municipalityCityName 直辖市对应的市级名称映射
// 直辖市的市级在 modood 数据中叫 "市辖区"，需要特殊处理
var municipalityCityName = map[string]string{
	"北京": "北京市",
	"天津": "天津市",
	"上海": "上海市",
	"重庆": "重庆市",
}

type locationCandidate struct {
	Name    string
	Id      int
	Pid     int
	Zipcode int
	Alias   string
	Pos     int
}

var (
	provinceAliasIndex map[string][]locationCandidate
	cityAliasIndex     map[string][]locationCandidate
	regionAliasIndex   map[string][]locationCandidate
)

func init() {
	buildAliasIndexes()
}

// Decompose 分离手机号(座机)，身份证号，姓名，地址等信息
func Decompose(info *Address, str string) *Address {
	//1. 过滤掉收货地址中的常用说明字符，排除干扰词（长词优先）
	for _, value := range FilterWord {
		str = strings.Replace(str, value, " ", -1)
	}

	//2. 多个空白字符(包括空格\r\n\t)换成一个空格
	reg := regexp.MustCompile(`\s+`)
	str = strings.TrimSpace(reg.ReplaceAllString(str, " "))

	//3. 去除手机号码中的短横线 如0136-3333-6666 主要针对苹果手机
	reg = regexp.MustCompile(`0?(\d{3})-(\d{4})-(\d{4})([-_]\d{2,})`)
	str = reg.ReplaceAllString(str, "$1$2$3$4")

	//4. 提取中国境内身份证号码
	reg = regexp.MustCompile(`(?i)\d{18}|\d{17}X`)
	IdNumber := reg.FindString(str)
	str = strings.Replace(str, IdNumber, "", -1)
	info.IdNumber = strings.ToUpper(IdNumber)

	//5. 提取手机号码或者座机号，优先完整提取带国际区号和分段写法的号码
	reg = regexp.MustCompile(`(?:\+\d{1,4}|00\d{1,4})[- ]?\d{6,14}(?:[-_]\d{2,6})?|1[3-9]\d[- ]?\d{4}[- ]?\d{4}(?:[-_]\d{2,6})?|(?:400|800)-\d{3}-\d{4}|(?:400|800)\d{7}|\d{3,4}-\d{6,8}|\d{7,11}[\-_]\d{2,6}|\d{7,11}`)
	mobileRaw := reg.FindString(str)
	str = strings.Replace(str, mobileRaw, "", 1)
	info.Mobile = normalizeMobile(mobileRaw)

	//6. 提取6位邮编（仅当邮编前后有明确标识或独立出现时才提取，避免误提取门牌号）
	// 优先匹配 "邮编XXXXXX" 或 "XXXXXX" 独立出现的情况
	postcode := ""
	regPostcode := regexp.MustCompile(`(?:邮编|邮政编码)\s*(\d{6})`)
	if m := regPostcode.FindStringSubmatch(str); len(m) > 1 {
		postcode = m[1]
		str = strings.Replace(str, m[0], "", -1)
	} else {
		// 回退：匹配独立的6位数字（前后不是数字，且不在地址中间）
		reg = regexp.MustCompile(`(?:^|\s)(\d{6})(?:\s|$)`)
		if m := reg.FindStringSubmatch(str); len(m) > 1 {
			postcode = m[1]
			str = strings.Replace(str, m[0], " ", -1)
		}
	}
	info.PostCode = postcode

	//再次把2个及其以上的空格合并成一个，并首位TRIM
	reg = regexp.MustCompile(` {2,}`)
	str = strings.TrimSpace(reg.ReplaceAllString(str, " "))

	//7. 按照空格切分 长度长的为地址 短的为姓名 因为不是基于自然语言分析，所以采取统计学上高概率的方案
	r := strings.Split(str, " ")

	name := r[0]
	for _, v := range r {
		if len(v) < len(name) {
			name = v
		}
	}

	if len(r) <= 1 {
		info.Address = r[0]
		return info
	}

	info.Name = name
	// 只替换第一个匹配，避免姓名和地址中有相同片段时被误删
	address := strings.TrimSpace(strings.Replace(str, name, "", 1))
	info.Address = address

	return info
}

// Smart 智能解析
func Smart(str string) *Address {
	var info Address
	info = *Decompose(&info, str)
	Parse(&info)

	// 提取街道地址：先按 City 提取（粗粒度），再按 Region 提取（细粒度覆盖）
	// 这样 Region 级别的提取不会被 City 覆盖
	if info.City != "" && info.City != "市辖区" && strings.Contains(info.Address, info.City) {
		info.Street = info.Address[strings.LastIndex(info.Address, info.City)+len(info.City):]
	} else if info.City != "" && info.City != "市辖区" {
		info.Street = suffixAfterLocation(info.Address, info.City, cityAliasIndex)
	}
	if info.Region != "" && strings.Contains(info.Address, info.Region) {
		info.Street = info.Address[strings.LastIndex(info.Address, info.Region)+len(info.Region):]
	} else if info.Region != "" {
		if street := suffixAfterLocation(info.Address, info.Region, regionAliasIndex); street != "" {
			info.Street = street
		}
	}

	return &info
}

// resolveProvinceAlias 解析省级别名，将 "北京市" 等映射到 "北京"
func resolveProvinceAlias(name string) string {
	if alias, ok := provinceAlias[name]; ok {
		return alias
	}
	return name
}

func normalizeMobile(mobile string) string {
	mobile = strings.TrimSpace(mobile)
	if strings.HasPrefix(mobile, "+86") {
		mobile = strings.TrimLeft(strings.TrimPrefix(mobile, "+86"), "- ")
	} else if strings.HasPrefix(mobile, "0086") {
		mobile = strings.TrimLeft(strings.TrimPrefix(mobile, "0086"), "- ")
	}
	if m := regexp.MustCompile(`^(1[3-9]\d)[- ]?(\d{4})[- ]?(\d{4})([-_]\d{2,6})?$`).FindStringSubmatch(mobile); len(m) > 0 {
		return m[1] + m[2] + m[3] + m[4]
	}
	return mobile
}

// Parse 智能解析出省市区+街道地址
func Parse(address *Address) *Address {
	addrStr := address.Address

	// 匹配所有省级
	pReg := regexp.MustCompile(`.+?(省|市|自治区|特别行政区|区)`)
	pArr := pReg.FindAllString(addrStr, -1)

	// 扩展省级候选：加入别名解析后的结果
	var pArrExpanded []string
	for _, p := range pArr {
		pArrExpanded = append(pArrExpanded, p)
		if alias := resolveProvinceAlias(p); alias != p {
			pArrExpanded = append(pArrExpanded, alias)
		}
	}

	// 匹配所有市级
	cReg := regexp.MustCompile(`.+?(省|市|自治州|州|地区|盟|县|自治县|区|林区)`)
	cArr := append(cReg.FindAllString(addrStr, -1), pArr...)

	// 匹配所有区县级
	rReg := regexp.MustCompile(`.+?(市|县|自治县|旗|自治旗|区|林区|特区|街道|镇|乡)`)
	rArr := append(rReg.FindAllString(addrStr, -1), cArr...)

	// 去重 rArr 和 cArr，保持顺序
	rArr = dedup(rArr)
	cArr = dedup(cArr)

	// 按长度降序排列候选，优先匹配更长的地名（更精确）
	sort.SliceStable(rArr, func(i, j int) bool { return len(rArr[i]) > len(rArr[j]) })
	sort.SliceStable(cArr, func(i, j int) bool { return len(cArr[i]) > len(cArr[j]) })

	if address.Province == "" {
		if candidate, ok := bestProvinceCandidate(addrStr); ok {
			applyProvinceCandidate(address, candidate)
		}
	}

	if address.City == "" {
		if candidate, ok := bestCityCandidate(addrStr, address); ok {
			applyCityCandidate(address, candidate)
		}
	}

	if candidate, ok := bestRegionCandidate(addrStr, address); ok {
		applyRegionCandidate(address, candidate)
	}

	// 处理区县级
I:
	for _, r := range rArr {
		if address.Region != "" {
			break
		}
		if r1, ok := areaMap.RegionByName[r]; ok && len(r1) == 1 {
			address.Region = r1[0].Name
			if r1[0].Zipcode != 0 {
				address.PostCode = strconv.Itoa(r1[0].Zipcode)
			}
			getAddressById(address, r1[0].Pid, city)
			break
		} else if ok {
			// 多个同名区县，尝试通过市级候选交叉验证
			for _, r2 := range r1 {
				address.Region = r2.Name
				if r2.Zipcode != 0 {
					address.PostCode = strconv.Itoa(r2.Zipcode)
				}
				getAddressById(address, r2.Pid, city)
				// 交叉验证：检查解析出的 City 是否在候选市级列表中
				for _, v := range cArr {
					if address.City == v {
						break I
					}
				}
				// 交叉验证：检查解析出的 Province 是否在候选省级列表中
				for _, v := range pArrExpanded {
					if address.Province == v {
						break I
					}
				}
			}
		}
	}

	// 处理市级
	if address.City == "" {
		for _, c := range cArr {
			if r1, ok := areaMap.CityByName[c]; ok {
				address.City = r1[0].Name
				if r1[0].Zipcode != 0 {
					address.PostCode = strconv.Itoa(r1[0].Zipcode)
				}
				getAddressById(address, r1[0].Pid, province)
				getAddressByPid(address, r1[0].Id, region, rArr)
				break
			}
		}
	}

	// 处理省级
	if address.Province == "" {
		for _, p := range pArrExpanded {
			if candidates := provinceCandidatesByAlias(p); len(candidates) > 0 {
				applyProvinceCandidate(address, candidates[0])
				getAddressByPid(address, candidates[0].Id, city, cArr)
				getAddressByPid(address, candidates[0].Id, region, rArr)
				break
			}
		}
	}

	// 直辖市特殊处理：如果省级已匹配但市级仍为 "市辖区" 或空，用直辖市名称填充
	if address.Province != "" && (address.City == "" || address.City == "市辖区") {
		if cityName, ok := municipalityCityName[address.Province]; ok {
			address.City = cityName
		}
	}

	return address
}

const (
	// 定义map等级常量
	province = "province"
	city     = "city"
	region   = "region"
)

// 根据id获取地址信息
func getAddressById(address *Address, id int, rank string) *Address {
	if rank == province {
		info := areaMap.ProvinceById[id]
		address.Province = info.Name
	}
	if rank == city {
		info := areaMap.CityById[id]
		address.City = info.Name
		getAddressById(address, info.Pid, province)
	}
	if rank == region {
		info := areaMap.RegionById[id]
		address.Region = info.Name
		getAddressById(address, info.Pid, city)
	}
	return address
}

// 根据pid获取下一级行政地址信息
func getAddressByPid(address *Address, pid int, rank string, arr []string) *Address {
	if rank == city && address.City == "" {
		for _, addr := range arr {
			for _, info := range areaMap.CityByPid[pid] {
				if strings.Contains(info.Name, addr) {
					address.City = info.Name
					if info.Zipcode != 0 {
						address.PostCode = strconv.Itoa(info.Zipcode)
					}
					return address
				}
			}
		}
	}
	if rank == region && address.Region == "" {
		for _, addr := range arr {
			for _, info := range areaMap.RegionByPid[pid] {
				if strings.Contains(info.Name, addr) {
					address.Region = info.Name
					if info.Zipcode != 0 {
						address.PostCode = strconv.Itoa(info.Zipcode)
					}
					return address
				}
			}
		}
	}
	return address
}

// dedup 去重并保持原始顺序
func dedup(arr []string) []string {
	seen := make(map[string]bool, len(arr))
	result := make([]string, 0, len(arr))
	for _, s := range arr {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func buildAliasIndexes() {
	provinceAliasIndex = make(map[string][]locationCandidate)
	cityAliasIndex = make(map[string][]locationCandidate)
	regionAliasIndex = make(map[string][]locationCandidate)

	for _, id := range sortedProvinceIDs() {
		info := areaMap.ProvinceById[id]
		candidate := locationCandidate{Name: info.Name, Id: id, Pid: info.Pid}
		for _, alias := range locationAliases(info.Name) {
			addAlias(provinceAliasIndex, alias, candidate)
		}
	}
	for alias, name := range provinceAlias {
		for _, candidate := range provinceCandidatesByAlias(name) {
			addAlias(provinceAliasIndex, alias, candidate)
		}
	}

	for _, id := range sortedCityIDs() {
		info := areaMap.CityById[id]
		if info.Name == "市辖区" || info.Name == "县" || strings.Contains(info.Name, "直辖县级行政区划") {
			continue
		}
		candidate := locationCandidate{Name: info.Name, Id: id, Pid: info.Pid, Zipcode: info.Zipcode}
		for _, alias := range locationAliases(info.Name) {
			addAlias(cityAliasIndex, alias, candidate)
		}
		for _, alias := range shortAutonomousAliases(info.Name) {
			addAlias(cityAliasIndex, alias, candidate)
		}
		for _, alias := range commonCityAliases(info.Name) {
			addAlias(cityAliasIndex, alias, candidate)
		}
	}

	for _, id := range sortedRegionIDs() {
		info := areaMap.RegionById[id]
		candidate := locationCandidate{Name: info.Name, Id: id, Pid: info.Pid, Zipcode: info.Zipcode}
		for _, alias := range locationAliases(info.Name) {
			addAlias(regionAliasIndex, alias, candidate)
		}
		for _, alias := range shortAutonomousAliases(info.Name) {
			addAlias(regionAliasIndex, alias, candidate)
		}
	}
}

func sortedProvinceIDs() []int {
	ids := make([]int, 0, len(areaMap.ProvinceById))
	for id := range areaMap.ProvinceById {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func sortedCityIDs() []int {
	ids := make([]int, 0, len(areaMap.CityById))
	for id := range areaMap.CityById {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func sortedRegionIDs() []int {
	ids := make([]int, 0, len(areaMap.RegionById))
	for id := range areaMap.RegionById {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func addAlias(index map[string][]locationCandidate, alias string, candidate locationCandidate) {
	alias = strings.TrimSpace(alias)
	if len([]rune(alias)) < 2 {
		return
	}
	for _, existing := range index[alias] {
		if existing.Id == candidate.Id {
			return
		}
	}
	candidate.Alias = alias
	index[alias] = append(index[alias], candidate)
}

func locationAliases(name string) []string {
	aliases := []string{name}
	short := trimAdministrativeSuffix(name)
	if short != name {
		aliases = append(aliases, short)
	}
	return dedup(aliases)
}

func trimAdministrativeSuffix(name string) string {
	suffixes := []string{
		"特别行政区", "维吾尔自治区", "壮族自治区", "回族自治区", "自治区",
		"自治州", "地区", "自治县", "自治旗", "林区", "特区", "省", "市", "县", "旗", "区", "盟",
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSuffix(name, suffix)
		}
	}
	return name
}

func shortAutonomousAliases(name string) []string {
	if !strings.Contains(name, "自治") {
		return nil
	}

	short := name
	nationalities := []string{"蒙古", "回族", "藏族", "维吾尔", "苗族", "彝族", "壮族", "布依族", "侗族", "瑶族", "白族", "土家族", "哈尼族", "哈萨克", "傣族", "黎族", "傈僳", "佤族", "畲族", "高山", "拉祜", "水族", "东乡", "纳西", "景颇", "柯尔克孜", "土族", "达斡尔", "仫佬", "羌族", "布朗", "撒拉", "毛南", "仡佬", "锡伯", "阿昌", "普米", "塔吉克", "怒族", "乌孜别克", "俄罗斯", "鄂温克", "德昂", "保安", "裕固", "京族", "塔塔尔", "独龙", "鄂伦春", "赫哲", "门巴", "珞巴", "基诺"}
	for _, nationality := range nationalities {
		if idx := strings.Index(short, nationality); idx > 0 {
			short = short[:idx]
			break
		}
	}
	if short == name || len([]rune(short)) < 2 {
		return nil
	}

	var aliases []string
	aliases = append(aliases, short)
	if strings.HasSuffix(name, "自治州") {
		aliases = append(aliases, short+"州")
	}
	if strings.HasSuffix(name, "自治县") {
		aliases = append(aliases, short+"县")
	}
	if strings.HasSuffix(name, "自治旗") {
		aliases = append(aliases, short+"旗")
	}
	return dedup(aliases)
}

func commonCityAliases(name string) []string {
	switch name {
	case "呼和浩特市":
		return []string{"呼市"}
	case "乌鲁木齐市":
		return []string{"乌市"}
	case "锡林郭勒盟":
		return []string{"锡盟"}
	case "阿拉善盟":
		return []string{"阿盟"}
	}
	return nil
}

func provinceCandidatesByAlias(alias string) []locationCandidate {
	if candidates, ok := provinceAliasIndex[alias]; ok {
		return candidates
	}
	if candidates, ok := areaMap.ProvinceByName[alias]; ok {
		result := make([]locationCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			result = append(result, locationCandidate{
				Name: candidate.Name,
				Id:   candidate.Id,
				Pid:  candidate.Pid,
			})
		}
		return result
	}
	return nil
}

func scanLocationCandidates(addrStr string, index map[string][]locationCandidate) []locationCandidate {
	var result []locationCandidate
	for alias, candidates := range index {
		pos := strings.Index(addrStr, alias)
		if pos < 0 {
			continue
		}
		for _, candidate := range candidates {
			candidate.Alias = alias
			candidate.Pos = pos
			result = append(result, candidate)
		}
	}
	return result
}

func bestProvinceCandidate(addrStr string) (locationCandidate, bool) {
	candidates := scanLocationCandidates(addrStr, provinceAliasIndex)
	if len(candidates) == 0 {
		return locationCandidate{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		si := provinceCandidateScore(candidates[i])
		sj := provinceCandidateScore(candidates[j])
		if si != sj {
			return si > sj
		}
		return betterTieBreak(candidates[i], candidates[j])
	})
	return candidates[0], true
}

func bestCityCandidate(addrStr string, address *Address) (locationCandidate, bool) {
	candidates := scanLocationCandidates(addrStr, cityAliasIndex)
	if address.Province != "" {
		candidates = filterCityCandidatesByProvince(candidates, address.Province)
	}
	if len(candidates) == 0 {
		return locationCandidate{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		si := cityCandidateScore(addrStr, address, candidates[i])
		sj := cityCandidateScore(addrStr, address, candidates[j])
		if si != sj {
			return si > sj
		}
		return betterTieBreak(candidates[i], candidates[j])
	})
	return candidates[0], true
}

func bestRegionCandidate(addrStr string, address *Address) (locationCandidate, bool) {
	candidates := scanLocationCandidates(addrStr, regionAliasIndex)
	if address.Province != "" {
		candidates = filterRegionCandidatesByProvince(candidates, address.Province)
	}
	if len(candidates) == 0 {
		return locationCandidate{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		si := regionCandidateScore(addrStr, address, candidates[i])
		sj := regionCandidateScore(addrStr, address, candidates[j])
		if si != sj {
			return si > sj
		}
		return betterTieBreak(candidates[i], candidates[j])
	})
	return candidates[0], true
}

func locationScore(candidate locationCandidate) int {
	score := len([]rune(candidate.Alias)) * 10
	if candidate.Alias == candidate.Name {
		score += 30
	}
	return score
}

func provinceCandidateScore(candidate locationCandidate) int {
	score := len([]rune(candidate.Alias)) * 10
	if candidate.Pos <= 2 {
		score += 100
	} else if candidate.Pos <= 6 {
		score += 50
	}
	if candidate.Alias == candidate.Name {
		score += 10
	}
	return score
}

func cityCandidateScore(addrStr string, address *Address, candidate locationCandidate) int {
	score := locationScore(candidate)
	if address.Province != "" && provinceNameByID(candidate.Pid) == address.Province {
		score += 100
	}
	if parent, ok := parentProvinceCandidate(candidate.Pid); ok && locationMentionedBefore(addrStr, parent, candidate.Pos, provinceAliasIndex) {
		score += 70
	}
	return score
}

func regionCandidateScore(addrStr string, address *Address, candidate locationCandidate) int {
	score := locationScore(candidate)
	cityInfo := areaMap.CityById[candidate.Pid]
	if address.City != "" && (address.City == cityInfo.Name || (provinceNameByID(cityInfo.Pid) == address.Province && municipalityCityName[address.Province] == address.City)) {
		score += 120
	}
	if locationMentionedBefore(addrStr, locationCandidate{Name: cityInfo.Name, Id: candidate.Pid, Pid: cityInfo.Pid}, candidate.Pos, cityAliasIndex) {
		score += 90
	}
	if address.Province != "" && provinceNameByID(cityInfo.Pid) == address.Province {
		score += 70
	}
	if parent, ok := parentProvinceCandidate(cityInfo.Pid); ok && locationMentionedBefore(addrStr, parent, candidate.Pos, provinceAliasIndex) {
		score += 40
	}
	return score
}

func filterCityCandidatesByProvince(candidates []locationCandidate, provinceName string) []locationCandidate {
	result := make([]locationCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if provinceNameByID(candidate.Pid) == provinceName {
			result = append(result, candidate)
		}
	}
	return result
}

func filterRegionCandidatesByProvince(candidates []locationCandidate, provinceName string) []locationCandidate {
	result := make([]locationCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		cityInfo := areaMap.CityById[candidate.Pid]
		if provinceNameByID(cityInfo.Pid) == provinceName {
			result = append(result, candidate)
		}
	}
	return result
}

func betterTieBreak(a, b locationCandidate) bool {
	if len([]rune(a.Alias)) != len([]rune(b.Alias)) {
		return len([]rune(a.Alias)) > len([]rune(b.Alias))
	}
	if a.Pos != b.Pos {
		return a.Pos < b.Pos
	}
	return a.Id < b.Id
}

func locationMentionedBefore(addrStr string, location locationCandidate, before int, index map[string][]locationCandidate) bool {
	for alias, candidates := range index {
		pos := strings.Index(addrStr, alias)
		if pos < 0 || pos > before {
			continue
		}
		for _, candidate := range candidates {
			if candidate.Id == location.Id {
				return true
			}
		}
	}
	return false
}

func parentProvinceCandidate(id int) (locationCandidate, bool) {
	info, ok := areaMap.ProvinceById[id]
	if !ok {
		return locationCandidate{}, false
	}
	return locationCandidate{Name: info.Name, Id: id, Pid: info.Pid}, true
}

func provinceNameByID(id int) string {
	if info, ok := areaMap.ProvinceById[id]; ok {
		return info.Name
	}
	return ""
}

func applyProvinceCandidate(address *Address, candidate locationCandidate) {
	address.Province = candidate.Name
}

func applyCityCandidate(address *Address, candidate locationCandidate) {
	address.City = candidate.Name
	if candidate.Zipcode != 0 {
		address.PostCode = strconv.Itoa(candidate.Zipcode)
	}
	getAddressById(address, candidate.Pid, province)
}

func applyRegionCandidate(address *Address, candidate locationCandidate) {
	address.Region = candidate.Name
	if candidate.Zipcode != 0 {
		address.PostCode = strconv.Itoa(candidate.Zipcode)
	}
	getAddressById(address, candidate.Pid, city)
}

func suffixAfterLocation(addrStr, canonicalName string, index map[string][]locationCandidate) string {
	bestEnd := -1
	for alias, candidates := range index {
		pos := strings.LastIndex(addrStr, alias)
		if pos < 0 {
			continue
		}
		for _, candidate := range candidates {
			if candidate.Name != canonicalName {
				continue
			}
			end := pos + len(alias)
			if end > bestEnd {
				bestEnd = end
			}
		}
	}
	if bestEnd < 0 || bestEnd > len(addrStr) {
		return ""
	}
	return addrStr[bestEnd:]
}
