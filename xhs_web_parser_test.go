package main

import "testing"

func TestParseXHSWebText(t *testing.T) {
	raw := `早C晚A真的别乱叠，我脸红了三天
浏览 12.8万 点赞 8420 评论 516 收藏 2104
https://www.xiaohongshu.com/explore/abc123

油皮夏天精简护肤顺序
1.9万赞 238评论 620收藏
https://www.xiaohongshu.com/explore/def456`

	posts, err := ParseXHSWebText(raw)
	if err != nil {
		t.Fatalf("ParseXHSWebText returned error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if posts[0].Title != "早C晚A真的别乱叠，我脸红了三天" {
		t.Fatalf("unexpected title: %s", posts[0].Title)
	}
	if posts[0].Views != 128000 || posts[0].Likes != 8420 || posts[0].Comments != 516 || posts[0].Favorites != 2104 {
		t.Fatalf("unexpected first metrics: %+v", posts[0])
	}
	if posts[1].Likes != 19000 || posts[1].Comments != 238 || posts[1].Favorites != 620 {
		t.Fatalf("unexpected second metrics: %+v", posts[1])
	}
}
