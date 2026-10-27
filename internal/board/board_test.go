package board

import (
	"testing"
	"time"
)

func TestHelpers(t *testing.T) {
	if Unix(time.Time{}) != 0 || Unix(time.Unix(42, 0)) != 42 {
		t.Error("Unix")
	}
	if ShortID("abcdefghij") != "abcdefgh" || ShortID("abc") != "abc" {
		t.Error("ShortID")
	}
	if (RepoInfo{Owner: "a", Name: "b"}).Slug() != "a/b" || (RepoInfo{Owner: "a"}).Slug() != "" {
		t.Error("Slug")
	}
}

func TestMergeModelsSortsByUSDThenName(t *testing.T) {
	got := MergeModels([]ModelCost{{Model: "b", USD: 1}, {Model: "a", USD: 1}}, []ModelCost{{Model: "c", USD: 5}, {Model: "a", USD: 1, Messages: 2}})
	if len(got) != 3 || got[0].Model != "c" || got[1].Model != "a" || got[1].USD != 2 || got[1].Messages != 2 || got[2].Model != "b" {
		t.Fatalf("%+v", got)
	}
}
