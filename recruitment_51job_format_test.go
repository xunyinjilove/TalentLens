package main

import (
	"strings"
	"testing"
)

func TestFormat51JobResumeShowsRequestedSectionsWithoutMetadata(t *testing.T) {
	raw := `{"resumeid":"123","email":"private@example.com","selfintro":"擅长客户沟通","jobintention":[{"expectposition":"销售经理","expectarea":[{"provincecity":"宁波","county":""},{"provincecity":"杭州","county":"西湖区"}],"displayexpectsalary":"9000-18000/月","seektype":"全职"}],"work":[{"compname":"甲公司","position":"销售","workdescribe":"负责客户开发"}],"project":[{"projectname":"区域项目","describe":"完成交付"}],"education":[{"schoolname":"某大学","degree":"本科","major":"市场营销"}],"certification":[{"cert":"英语六级","getdate":"1900.01"}]}`
	got := format51JobResume(raw)
	for _, want := range []string{"求职意向", "个人优势", "工作经历", "项目经验", "教育经历", "证书", "本科", "英语六级", "宁波/杭州-西湖区"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	for _, secret := range []string{"private@example.com", "resumeid", `"work"`, "map[", "1900.01"} {
		if strings.Contains(got, secret) {
			t.Fatalf("metadata %q leaked in %q", secret, got)
		}
	}

	legacy := "求职意向\n销售工程师 · [map[county: provincecity:宁波]] · 9000-18000/月 · 全职"
	cleaned := format51JobResume(legacy)
	if strings.Contains(cleaned, "map[") || !strings.Contains(cleaned, "销售工程师 · 宁波 · 9000-18000/月 · 全职") {
		t.Fatalf("legacy map artifact not cleaned: %q", cleaned)
	}
}
