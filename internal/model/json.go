package model

import (
	"encoding/json"
	"fmt"
)

// requestFields is Request without its methods, so encoding/json doesn't
// recurse into Request.MarshalJSON.
type requestFields Request

// requestJSON is how history stores a request. Its Payload field shadows
// the interface field of the same name in requestFields, which encoding/json
// can't decode. Kind and Payload are left out for HTTP, so HTTP requests
// encode exactly as they did before kinds existed and old history loads as
// HTTP.
type requestJSON struct {
	requestFields
	Kind    KindID          `json:",omitempty"`
	Payload json.RawMessage `json:",omitempty"`
}

func (r Request) MarshalJSON() ([]byte, error) {
	out := requestJSON{requestFields: requestFields(r)}
	if r.Payload != nil {
		payload, err := json.Marshal(r.Payload)
		if err != nil {
			return nil, err
		}
		out.Kind, out.Payload = r.Kind().ID, payload
	}
	return json.Marshal(out)
}

func (r *Request) UnmarshalJSON(data []byte) error {
	var in requestJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	req := Request(in.requestFields)
	req.Payload = nil
	if in.Kind != "" {
		kind, ok := KindByID(in.Kind)
		if !ok {
			return fmt.Errorf("unsupported request kind %q", in.Kind)
		}
		if kind.decode != nil {
			payload, err := kind.decode(in.Payload)
			if err != nil {
				return fmt.Errorf("decoding %s request: %w", kind.Label, err)
			}
			req.Payload = payload
		}
	}
	*r = req
	return nil
}
