package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestCaptureValues(t *testing.T) {
	body := []byte(`{"data":{"id":9007199254740993,"items":[{"id":"first"},{"id":"${literal}$cash"}],"empty":"","enabled":false,"nothing":null,"a.b":{"quote\"key":[7,8]}}}`)
	captures := map[string]string{
		"ID": "data.id", "ITEM": "$.data.items[1].id", "EMPTY": "data.empty",
		"BOOL": "data.enabled", "NULL": "data.nothing", "ARRAY": `data["a.b"]["quote\"key"]`,
		"OBJECT": "data.items[0]",
	}
	want := map[string]string{
		"ID": "9007199254740993", "ITEM": "${literal}$cash", "EMPTY": "",
		"BOOL": "false", "NULL": "null", "ARRAY": "[7,8]", "OBJECT": `{"id":"first"}`,
	}
	values, err := CaptureValues(captures, body)
	if err != nil || !reflect.DeepEqual(values, want) {
		t.Fatalf("captures = %v, %v; want %v", values, err, want)
	}
	for _, path := range []string{"[1].id", "$[1].id"} {
		values, err := CaptureValues(map[string]string{"ID": path}, []byte(`[{"id":1},{"id":2}]`))
		if err != nil || values["ID"] != "2" {
			t.Fatalf("root array: %v, %v", values, err)
		}
	}
	values, err = CaptureValues(map[string]string{"VALUE": "$"}, []byte(`"root"`))
	if err != nil || values["VALUE"] != "root" {
		t.Fatalf("root capture: %v, %v", values, err)
	}
}

func TestCaptureJSONPath(t *testing.T) {
	body := []byte(`{
		"data": {"users": [
			{"id": 12, "role": "member", "active": false},
			{"id": 9007199254740993, "role": "admin", "active": true},
			{"id": 27}
		]},
		"nested": {"session": {"token": "secret"}},
		"one": {"item": [7, 8]},
		"unusual.key": {"": "empty key"},
		"日本語": {"id": "unicode"},
		"numbers": [12345678901234567890, 1.234567890123456789, 1e400]
	}`)
	for _, tc := range []struct{ path, want string }{
		{`data.users[?(@.role == "admin")].id`, "9007199254740993"},
		{`$.data.users[?(@.active == true && @.id > 20)].id`, "9007199254740993"},
		{`data.users[?(match(@.role, "admin"))].id`, "9007199254740993"},
		{`data.users[?(@.id == 12)].id`, "12"},
		{`data.users[-1].id`, "27"},
		{`data.users[1:2].id`, "9007199254740993"},
		{`$..token`, "secret"},
		{`nested..token`, "secret"},
		{`one.*`, "[7,8]"},
		{`["unusual.key"]['']`, "empty key"},
		{`日本語.id`, "unicode"},
		{`  $.日本語.id  `, "unicode"},
		{`numbers`, "[12345678901234567890,1.234567890123456789,1e400]"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			values, err := CaptureValues(map[string]string{"VALUE": tc.path}, body)
			if err != nil || values["VALUE"] != tc.want {
				t.Fatalf("capture = %v, %v; want %q", values, err, tc.want)
			}
		})
	}
}

func TestCaptureFailureReturnsNoPartialValues(t *testing.T) {
	for _, tc := range []struct{ body, path string }{
		{`{"data":{"id":1}}`, "data.missing"},
		{`{"data":{"id":1}}`, "data.id.nested"},
		{`{"data":{"id":1},"items":[0,1]}`, "items[2]"},
		{`{"data":{"id":1},"items":{}}`, "items[0]"},
		{`{"data":{"id":1},"items":[{"id":2},{"id":3}]}`, "items[*].id"},
		{`{"data":{"id":1},"items":[{"id":2},{"id":3}]}`, "items[?(@.id > 3)].id"},
		{`{"data":{"id":1}}`, "data['id','id']"},
		{`{"data":{"id":1}} {}`, "data.id"},
		{`<html>not JSON</html>`, "data.id"},
		{``, "data.id"},
	} {
		values, err := CaptureValues(map[string]string{"A_FIRST": "data.id", "Z_LAST": tc.path}, []byte(tc.body))
		if err == nil || values != nil {
			t.Fatalf("%s / %s: partial captures = %v, %v", tc.body, tc.path, values, err)
		}
	}
	if values, err := CaptureValues(nil, []byte("not JSON")); err != nil || len(values) != 0 {
		t.Fatal("requests without captures must accept non-JSON responses")
	}
}

func TestCaptureValidation(t *testing.T) {
	for _, path := range []string{
		"", " ", "data.", "data.[0]", "data[0]id", "$data", "$.data[", "data[01]",
		"data[?(@.id ==)]", ".data", "..id", "@.id", "*", "1id",
		`data["bad\x"]`, "data[99999999999999999999999999]",
	} {
		if err := ValidateCaptures(map[string]string{"ID": path}); err == nil || !strings.Contains(err.Error(), "ID") {
			t.Errorf("path %q: expected named validation error, got %v", path, err)
		}
	}
	for _, name := range []string{"", "1ID", "bad-name", "$uuid"} {
		if err := ValidateCaptures(map[string]string{name: "data.id"}); err == nil {
			t.Errorf("accepted capture name %q", name)
		}
	}
}
