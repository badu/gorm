package tests

import "testing"

func PrefixColumnNameForEmbeddedStruct(t *testing.T) {
	TestDB.NewScope(&EngadgetPost{})
	dialect := TestDB.Dialect()
	if !dialect.HasColumn(TestDB.NewScope(&EngadgetPost{}).TableName(), "author_name") || !dialect.HasColumn(TestDB.NewScope(&EngadgetPost{}).TableName(), "author_email") {
		t.Errorf("should has prefix for embedded columns")
	}

	if !dialect.HasColumn(TestDB.NewScope(&HNPost{}).TableName(), "user_name") || !dialect.HasColumn(TestDB.NewScope(&HNPost{}).TableName(), "user_email") {
		t.Errorf("should has prefix for embedded columns")
	}
}

func SaveAndQueryEmbeddedStruct(t *testing.T) {
	TestDB.Save(&HNPost{BasePost: BasePost{Title: "news"}})
	TestDB.Save(&HNPost{BasePost: BasePost{Title: "hn_news"}})
	var news HNPost
	if err := TestDB.First(&news, "title = ?", "hn_news").Error; err != nil {
		t.Errorf("no error should happen when query with embedded struct, but got %v", err)
	} else if news.Title != "hn_news" {
		t.Errorf("embedded struct's value should be scanned correctly")
	}

	TestDB.Save(&EngadgetPost{BasePost: BasePost{Title: "engadget_news"}})
	var egNews EngadgetPost
	if err := TestDB.First(&egNews, "title = ?", "engadget_news").Error; err != nil {
		t.Errorf("no error should happen when query with embedded struct, but got %v", err)
	} else if egNews.BasePost.Title != "engadget_news" {
		t.Errorf("embedded struct's value should be scanned correctly")
	}

	if TestDB.NewScope(&HNPost{}).PK() == nil {
		t.Errorf("primary key with embedded struct should works")
	}

	for _, field := range TestDB.NewScope(&HNPost{}).Fields() {
		if field.StructName == "BasePost" {
			t.Errorf("scope Fields should not contain embedded struct")
		}
	}
}
