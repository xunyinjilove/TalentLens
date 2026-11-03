package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// format51JobResume keeps platform metadata out of the readable resume while
// retaining the original JSON separately for rule evaluation and audit.
func format51JobResume(raw string) string {
	var data map[string]interface{}
	if json.Unmarshal([]byte(raw), &data) != nil || data["resumeid"] == nil {
		return raw
	}
	value := func(row map[string]interface{}, keys ...string) string {
		for _, key := range keys {
			if v, ok := row[key]; ok && v != nil {
				s := strings.TrimSpace(fmt.Sprint(v))
				if s != "" && s != "<nil>" && s != "0" {
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
	if line := join(value(data, "username", "name"), value(data, "sex"), value(data, "displayage"), value(data, "workyear")); line != "" {
		basic = append(basic, line)
	}
	if location := value(data, "area", "areaprovincecity"); location != "" {
		basic = append(basic, "现居地："+location)
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
		line := join(value(row, "cert"), value(row, "getdate"), value(row, "score"))
		if line != "" {
			certificates = append(certificates, line)
		}
	}
	section("证书", certificates)
	return strings.TrimSpace(strings.Join(output, "\n"))
}
