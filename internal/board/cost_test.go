package board

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBoardCostSumsSessionsByModel(t *testing.T) {
	s1 := SessionView{Cost: SessionCost{ByModel: []ModelCost{{Model: "a", USD: 1, Messages: 1}, {Model: "b", USD: 5, Messages: 2}}}}
	s2 := SessionView{Cost: SessionCost{ByModel: []ModelCost{{Model: "a", USD: 10, Messages: 3}}}}
	bc := TotalCost([]SessionView{s1, s2}, "2026-10-22")
	if bc.USD != 16 || bc.ByModel[0].Model != "a" || bc.ByModel[0].Messages != 4 {
		t.Fatalf("board = %+v", bc)
	}
}

func TestSessionCostJSONContract(t *testing.T) {
	c := SessionCost{USD: 1.5, PricedAsOf: "2026-10-22", ByModel: []ModelCost{{Model: "m", USD: 1.5, Messages: 2, Unpriced: true}}}
	b, _ := json.Marshal(c)
	for _, k := range []string{`"usd"`, `"partial"`, `"priced_as_of"`, `"tokens"`, `"by_model"`, `"cache_write_5m"`, `"cache_write_1h"`, `"cache_read"`, `"unpriced":true`} {
		if !strings.Contains(string(b), k) {
			t.Errorf("json %s lacks %s", b, k)
		}
	}
}
