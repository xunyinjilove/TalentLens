package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	goMapSliceArtifactPattern = regexp.MustCompile(`\[(?:\s*map\[[^\]]*\]\s*)+\]|\bmap\[[^\]]*\]`)
	goMapEntryPattern         = regexp.MustCompile(`map\[([^\]]*)\]`)
	provinceCityFieldPattern  = regexp.MustCompile(`provincecity:([^\s\]]+)`)
	countyFieldPattern        = regexp.MustCompile(`county:([^\s\]]+)`)
)

func cleanGoMapArtifacts(raw string) string {
	if !strings.Contains(raw, "map[") {
		return raw
	}
	cleaned := goMapSliceArtifactPattern.ReplaceAllStringFunc(raw, func(match string) string {
		entries := goMapEntryPattern.FindAllStringSubmatch(match, -1)
		var areas []string
		for _, entry := range entries {
			body := entry[1]
			pc := ""
			county := ""
			if m := provinceCityFieldPattern.FindStringSubmatch(body); len(m) > 1 {
				pc = strings.TrimSpace(m[1])
			}
			if m := countyFieldPattern.FindStringSubmatch(body); len(m) > 1 {
				county = strings.TrimSpace(m[1])
			}
			area := pc
			if pc != "" && county != "" && county != pc {
				area = pc + "-" + county
			} else if area == "" {
				area = county
			}
			if area != "" {
				areas = append(areas, area)
			}
		}
		return strings.Join(areas, "/")
	})
	cleaned = strings.ReplaceAll(cleaned, " ·  · ", " · ")
	cleaned = strings.ReplaceAll(cleaned, "现居地：现居·", "现居地：")
	return cleaned
}

// format51JobResume keeps platform metadata out of the readable resume while
// retaining the original JSON separately for rule evaluation and audit.
func format51JobResume(raw string) string {
	var data map[string]interface{}
	if json.Unmarshal([]byte(raw), &data) != nil || data["resumeid"] == nil {
		return cleanGoMapArtifacts(raw)
	}
	var stringifyValue func(v interface{}) string
	stringifyValue = func(v interface{}) string {
		if v == nil {
			return ""
		}
		switch val := v.(type) {
		case string:
			s := strings.TrimSpace(val)
			if s == "" || s == "<nil>" || s == "0" || s == "[]" || s == "{}" {
				return ""
			}
			return s
		case []interface{}:
			parts := make([]string, 0, len(val))
			for _, item := range val {
				if s := stringifyValue(item); s != "" {
					parts = append(parts, s)
				}
			}
			return strings.Join(parts, "/")
		case map[string]interface{}:
			pc, _ := val["provincecity"].(string)
			county, _ := val["county"].(string)
			pc = strings.TrimSpace(pc)
			county = strings.TrimSpace(county)
			if pc != "" && county != "" && county != pc {
				return pc + "-" + county
			}
			if pc != "" {
				return pc
			}
			if county != "" {
				return county
			}
			for _, k := range []string{"name", "label", "value", "text", "title"} {
				if s := stringifyValue(val[k]); s != "" {
					return s
				}
			}
			return ""
		default:
			s := strings.TrimSpace(fmt.Sprint(v))
			if s == "" || s == "<nil>" || s == "0" || strings.HasPrefix(s, "map[") || strings.HasPrefix(s, "[map[") {
				return ""
			}
			return s
		}
	}
	value := func(row map[string]interface{}, keys ...string) string {
		for _, key := range keys {
			if v, ok := row[key]; ok && v != nil {
				if s := stringifyValue(v); s != "" {
					return s
				}
			}
		}
		return ""
	}
	items := func(key string) []map[string]interface{} {
		list, _ := data[key].([]interface{})
		result := make([]map[string]interface{}, 0, len(list))
		for _, item := range list {
			if row, ok := item.(map[string]interface{}); ok {
				result = append(result, row)
			}
		}
		return result
	}
	var output []string
	section := func(title string, lines []string) {
		output = append(output, title)
		if len(lines) == 0 {
			output = append(output, "未填写")
		} else {
			output = append(output, lines...)
		}
		output = append(output, "")
	}
	join := func(parts ...string) string {
		clean := make([]string, 0, len(parts))
		for _, part := range parts {
			if strings.TrimSpace(part) != "" {
				clean = append(clean, strings.TrimSpace(part))
			}
		}
		return strings.Join(clean, " · ")
	}
	basic := []string{}
	workyear := value(data, "workyear")
	if workyear != "" && !strings.Contains(workyear, "年") && !strings.Contains(workyear, "经验") {
		workyear = workyear + "年经验"
	}
	if line := join(value(data, "username", "name"), value(data, "sex"), value(data, "displayage"), workyear); line != "" {
		basic = append(basic, line)
	}
	if location := value(data, "area", "areaprovincecity"); location != "" {
		location = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(location, "现居·"), "现居："), "现居:"))
		if location != "" {
			basic = append(basic, "现居地："+location)
		}
	}
	section("基本信息", basic)
	intention := []string{}
	for _, row := range items("jobintention") {
		line := join(value(row, "expectposition", "expectfuncname"), value(row, "expectarea"), value(row, "displayexpectsalary", "newdisplayexpectsalary"), value(row, "seektype"))
		if line != "" {
			intention = append(intention, line)
		}
	}
	section("求职意向", intention)
	advantages := []string{}
	if intro := value(data, "selfintro"); intro != "" {
		advantages = append(advantages, intro)
	}
	section("个人优势", advantages)
	work := []string{}
	for _, row := range items("work") {
		head := join(value(row, "timefrom")+"—"+value(row, "timeto"), value(row, "compname"), value(row, "position"))
		if head != "—" {
			work = append(work, head)
		}
		if detail := value(row, "workdescribe"); detail != "" {
			work = append(work, detail)
		}
	}
	section("工作经历", work)
	projects := []string{}
	for _, row := range items("project") {
		head := join(value(row, "timefrom")+"—"+value(row, "timeto"), value(row, "projectname"), value(row, "compname"))
		if head != "—" {
			projects = append(projects, head)
		}
		if detail := value(row, "describe"); detail != "" {
			projects = append(projects, detail)
		}
	}
	section("项目经验", projects)
	education := []string{}
	for _, row := range items("education") {
		line := join(value(row, "timefrom")+"—"+value(row, "timeto"), value(row, "schoolname"), value(row, "major"), value(row, "degree"))
		if line != "—" {
			education = append(education, line)
		}
	}
	section("教育经历", education)
	certificates := []string{}
	for _, row := range items("certification") {
		getdate := value(row, "getdate")
		if strings.HasPrefix(getdate, "1900") {
			getdate = ""
		}
		line := join(value(row, "cert"), getdate, value(row, "score"))
		if line != "" {
			certificates = append(certificates, line)
		}
	}
	section("证书", certificates)
	return strings.TrimSpace(strings.Join(output, "\n"))
}
