package client

import (
	"bytes"
	"encoding/json"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// templateDepth is how deep nested messages are filled in. Deeper ones, and
// recursive ones, are left as {}.
const templateDepth = 3

// template is a JSON message of type msg with a placeholder for every
// field: zero scalars, an enum's first non-zero value, one element for
// repeated fields and maps, and only the first field of each oneof, since
// setting two is an error. Well-known types take their JSON forms.
func template(msg protoreflect.MessageDescriptor) string {
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(messageTemplate(msg, 0)), "", "  "); err != nil {
		return "{}"
	}
	return out.String()
}

func messageTemplate(msg protoreflect.MessageDescriptor, depth int) string {
	if wkt, ok := wellKnownTemplates[msg.FullName()]; ok {
		return wkt
	}
	if depth >= templateDepth {
		return "{}"
	}
	var parts []string
	fields := msg.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if oneof := fd.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() && oneof.Fields().Get(0) != fd {
			continue
		}
		parts = append(parts, jsonString(fd.JSONName())+":"+fieldTemplate(fd, depth))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func fieldTemplate(fd protoreflect.FieldDescriptor, depth int) string {
	switch {
	case fd.IsMap():
		return "{" + mapKeyTemplate(fd.MapKey()) + ":" + singularTemplate(fd.MapValue(), depth) + "}"
	case fd.IsList():
		return "[" + singularTemplate(fd, depth) + "]"
	}
	return singularTemplate(fd, depth)
}

func singularTemplate(fd protoreflect.FieldDescriptor, depth int) string {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return "false"
	case protoreflect.StringKind, protoreflect.BytesKind:
		return `""`
	case protoreflect.EnumKind:
		return enumTemplate(fd.Enum())
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return messageTemplate(fd.Message(), depth+1)
	}
	return "0"
}

// mapKeyTemplate is a map key protojson accepts for the key's type.
func mapKeyTemplate(fd protoreflect.FieldDescriptor) string {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return `""`
	case protoreflect.BoolKind:
		return `"false"`
	}
	return `"0"`
}

func enumTemplate(enum protoreflect.EnumDescriptor) string {
	if enum.FullName() == "google.protobuf.NullValue" {
		return "null"
	}
	values := enum.Values()
	if values.Len() == 0 {
		return "0"
	}
	for i := range values.Len() {
		if values.Get(i).Number() != 0 {
			return jsonString(string(values.Get(i).Name()))
		}
	}
	return jsonString(string(values.Get(0).Name()))
}

var wellKnownTemplates = map[protoreflect.FullName]string{
	"google.protobuf.Timestamp":   `"1970-01-01T00:00:00Z"`,
	"google.protobuf.Duration":    `"0s"`,
	"google.protobuf.FieldMask":   `""`,
	"google.protobuf.Struct":      `{}`,
	"google.protobuf.Value":       `null`,
	"google.protobuf.ListValue":   `[]`,
	"google.protobuf.Empty":       `{}`,
	"google.protobuf.Any":         `{}`,
	"google.protobuf.DoubleValue": `0`,
	"google.protobuf.FloatValue":  `0`,
	"google.protobuf.Int64Value":  `0`,
	"google.protobuf.UInt64Value": `0`,
	"google.protobuf.Int32Value":  `0`,
	"google.protobuf.UInt32Value": `0`,
	"google.protobuf.BoolValue":   `false`,
	"google.protobuf.StringValue": `""`,
	"google.protobuf.BytesValue":  `""`,
}

func jsonString(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}
