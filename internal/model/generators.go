package model

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"
)

// Generators describes the reserved ${$name} variables. Descriptions, rather
// than sample values, are shown while editing: generation only happens when
// preparing a send (or a resolved command export).
var Generators = []struct {
	Name, Description string
	generate          func(time.Time) string
}{
	{"$uuid", "UUID v4 · fresh on every send", func(time.Time) string {
		var b [16]byte
		rand.Read(b[:])
		b[6] = b[6]&0x0f | 0x40
		b[8] = b[8]&0x3f | 0x80
		return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
	}},
	{"$timestamp", "Unix seconds · evaluated on send", func(now time.Time) string {
		return strconv.FormatInt(now.Unix(), 10)
	}},
	{"$timestampMs", "Unix milliseconds · evaluated on send", func(now time.Time) string {
		return strconv.FormatInt(now.UnixMilli(), 10)
	}},
	{"$isoTimestamp", "UTC ISO 8601 time · evaluated on send", func(now time.Time) string {
		return now.UTC().Format(time.RFC3339Nano)
	}},
	{"$randomInt", "Random integer 0–2147483647 · fresh on send", func(time.Time) string {
		var b [4]byte
		rand.Read(b[:])
		return strconv.FormatUint(uint64(binary.BigEndian.Uint32(b[:])&0x7fffffff), 10)
	}},
}

func generatedVariables() []Variable {
	now := time.Now()
	variables := make([]Variable, 0, len(Generators))
	for _, g := range Generators {
		variables = append(variables, Variable{Name: g.Name, Value: g.generate(now), Source: "generated"})
	}
	return variables
}
