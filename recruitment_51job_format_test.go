package main

import (
	"strings"
	"testing"
)

func TestFormat51JobResumeShowsRequestedSectionsWithoutMetadata(t *testing.T) {
	raw := `{"resumeid":"123","email":"private@example.com","selfintro":"擅长客户沟通","jobintention":[{"expectposition":"销售经理","expectarea":"成都"}],"work":[{"compname":"甲公司","position":"销售","workdescribe":"负责客户开发"}],"project":[{"projectname":"区域项目","describe":"完成交付"}],"education":[{"schoolname":"某大学","degree":"本科","major":"市场营销"}],"certification":[{"cert":"英语六级"}]}`
	got := format51JobResume(raw)
	for _, want := range []string{"求职意向", "个人优势", "工作经历", "项目经验", "教育经历", "证书", "本科", "英语六级"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	for _, secret := range []string{"private@example.com", "resumeid", `"work"`} {
		if strings.Contains(got, secret) {
			t.Fatalf("metadata %q leaked", secret)
		}
	}
}
