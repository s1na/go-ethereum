package main

import (
	"encoding/json"
)

func Result() json.RawMessage {
	return json.RawMessage(`{"foo": "bar"}`)
}
