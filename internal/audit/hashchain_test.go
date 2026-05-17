package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestCanonicalize_OrderIndependent(t *testing.T) {
	a := json.RawMessage(`{"b":1,"a":2}`)
	b := json.RawMessage(`{"a":2,"b":1}`)
	ca, err := Canonicalize(a)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := Canonicalize(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca, cb) {
		t.Fatalf("canonicalization not order-independent: %s vs %s", ca, cb)
	}
}

func TestCanonicalize_Nested(t *testing.T) {
	a := json.RawMessage(`{"x":{"b":1,"a":2},"arr":[3,1,2]}`)
	b := json.RawMessage(`{"arr":[3,1,2],"x":{"a":2,"b":1}}`)
	ca, _ := Canonicalize(a)
	cb, _ := Canonicalize(b)
	if !bytes.Equal(ca, cb) {
		t.Fatalf("nested canonicalization mismatch:\n%s\n%s", ca, cb)
	}
}

func TestCanonicalize_ArrayOrderPreserved(t *testing.T) {
	out, err := Canonicalize(json.RawMessage(`[3,1,2]`))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "[3,1,2]" {
		t.Fatalf("array order must be preserved, got %s", out)
	}
}

func TestCanonicalize_EmptyRaw(t *testing.T) {
	out, err := Canonicalize(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "null" {
		t.Fatalf("want null, got %s", out)
	}
}

func TestCanonicalize_RejectsInvalid(t *testing.T) {
	if _, err := Canonicalize(json.RawMessage(`{not json`)); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHash_DeterministicAcrossKeyOrder(t *testing.T) {
	e1 := Entry{
		ID:       1,
		TS:       1000,
		UserID:   "u",
		Action:   "a",
		Resource: "r",
		Details:  json.RawMessage(`{"b":1,"a":2}`),
		PrevHash: GenesisHash,
	}
	e2 := e1
	e2.Details = json.RawMessage(`{"a":2,"b":1}`)
	h1, err := Hash(e1)
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := Hash(e2)
	if h1 != h2 {
		t.Fatal("hash depends on JSON key order; canonicalization broken")
	}
}

func TestHash_ChangesOnEveryField(t *testing.T) {
	base := Entry{
		ID:       1,
		TS:       1000,
		UserID:   "u",
		Action:   "a",
		Resource: "r",
		Details:  json.RawMessage(`{}`),
		PrevHash: GenesisHash,
	}
	h0, err := Hash(base)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(e *Entry){
		"id":        func(e *Entry) { e.ID = 2 },
		"ts":        func(e *Entry) { e.TS = 2000 },
		"user_id":   func(e *Entry) { e.UserID = "v" },
		"action":    func(e *Entry) { e.Action = "b" },
		"resource":  func(e *Entry) { e.Resource = "s" },
		"details":   func(e *Entry) { e.Details = json.RawMessage(`{"x":1}`) },
		"prev_hash": func(e *Entry) { e.PrevHash[0] ^= 0xff },
	}
	for name, mut := range mutations {
		t.Run(name, func(t *testing.T) {
			e := base
			mut(&e)
			h, _ := Hash(e)
			if h == h0 {
				t.Fatalf("hash unchanged after mutating %s", name)
			}
		})
	}
}

func TestVerify_OK(t *testing.T) {
	e := Entry{ID: 1, TS: 1, UserID: "u", Action: "a", Resource: "r", Details: json.RawMessage(`{}`), PrevHash: GenesisHash}
	h, _ := Hash(e)
	if err := Verify(GenesisHash, e, h); err != nil {
		t.Fatal(err)
	}
}

func TestVerify_PrevHashMismatch(t *testing.T) {
	e := Entry{ID: 1, TS: 1, UserID: "u", Action: "a", Resource: "r", Details: json.RawMessage(`{}`), PrevHash: GenesisHash}
	h, _ := Hash(e)
	bogus := GenesisHash
	bogus[0] ^= 0xff
	err := Verify(bogus, e, h)
	if !errors.Is(err, ErrChainBroken) {
		t.Fatalf("want ErrChainBroken, got %v", err)
	}
}

func TestVerify_EntryHashMismatch(t *testing.T) {
	e := Entry{ID: 1, TS: 1, UserID: "u", Action: "a", Resource: "r", Details: json.RawMessage(`{}`), PrevHash: GenesisHash}
	h, _ := Hash(e)
	h[0] ^= 0xff
	err := Verify(GenesisHash, e, h)
	if !errors.Is(err, ErrChainBroken) {
		t.Fatalf("want ErrChainBroken, got %v", err)
	}
}

func TestVerify_ChainOfTwo(t *testing.T) {
	e1 := Entry{ID: 1, TS: 1, UserID: "u", Action: "a", Resource: "r", Details: json.RawMessage(`{"v":1}`), PrevHash: GenesisHash}
	h1, _ := Hash(e1)
	e2 := Entry{ID: 2, TS: 2, UserID: "u", Action: "a", Resource: "r", Details: json.RawMessage(`{"v":2}`), PrevHash: h1}
	h2, _ := Hash(e2)
	if err := Verify(GenesisHash, e1, h1); err != nil {
		t.Fatal(err)
	}
	if err := Verify(h1, e2, h2); err != nil {
		t.Fatal(err)
	}
}
