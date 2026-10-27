package registry

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDuplicateRegistrationPanics(t *testing.T) {
	d := func(Resolver, json.RawMessage, int) (RuntimeSettings, error) { return nil, nil }
	RegisterRuntime("dup-test", d)
	defer func() {
		if recover() == nil {
			t.Fatal("second registration of one name must panic")
		}
	}()
	RegisterRuntime("dup-test", d)
}

func TestNamesAreSortedAndLookupWorks(t *testing.T) {
	d := func(Resolver, json.RawMessage, int) (AgentSettings, error) { return nil, nil }
	RegisterAgent("zz-agent", d)
	RegisterAgent("aa-agent", d)
	names := AgentNames()
	if strings.Index(names, "aa-agent") > strings.Index(names, "zz-agent") || strings.Index(names, "aa-agent") < 0 {
		t.Fatalf("names %q", names)
	}
	if _, ok := Agent("aa-agent"); !ok {
		t.Fatal("registered agent not found")
	}
	if _, ok := Agent("nope"); ok {
		t.Fatal("unknown agent found")
	}
}

func TestStrictRefusesUnknownKeysAndTrailingData(t *testing.T) {
	var v struct {
		A int `json:"a"`
	}
	if err := Strict([]byte(`{"a":1}`), &v); err != nil || v.A != 1 {
		t.Fatalf("%v %+v", err, v)
	}
	if Strict([]byte(`{"a":1,"b":2}`), &v) == nil {
		t.Fatal("unknown key accepted")
	}
	if Strict([]byte(`{"a":1} {"a":2}`), &v) == nil {
		t.Fatal("trailing data accepted")
	}
}

func TestTypeOfNeedsAType(t *testing.T) {
	if _, err := TypeOf(json.RawMessage(`{}`)); err == nil {
		t.Fatal("missing type accepted")
	}
	if got, err := TypeOf(json.RawMessage(`{"type":"x"}`)); err != nil || got != "x" {
		t.Fatalf("%q %v", got, err)
	}
}
