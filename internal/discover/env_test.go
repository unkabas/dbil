package discover

import "testing"

func TestParseEnvJSON_HappyPath(t *testing.T) {
	raw := `[
	  {"alias":"app","host":"postgres","port":5432,"user":"app","password":"s","database":"appdb","tag":"dev"},
	  {"alias":"staging","host":"stg","port":6432,"username":"u","password":"x","database":"d","tag":"staging"}
	]`
	out, err := ParseEnvJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("count: %d", len(out))
	}
	if out[0].Alias != "app" || out[0].Username != "app" || out[0].Source != SourceEnv {
		t.Fatalf("first: %+v", out[0])
	}
	if out[1].Username != "u" || out[1].Tag != "staging" {
		t.Fatalf("second: %+v", out[1])
	}
}

func TestParseEnvJSON_EmptyReturnsNil(t *testing.T) {
	out, err := ParseEnvJSON("")
	if err != nil || out != nil {
		t.Fatalf("got %v, %v", out, err)
	}
	out, err = ParseEnvJSON("   ")
	if err != nil || out != nil {
		t.Fatalf("ws-only: %v, %v", out, err)
	}
}

func TestParseEnvJSON_TagDefaultDev(t *testing.T) {
	out, err := ParseEnvJSON(`[{"alias":"a","host":"h","port":5432,"user":"u","database":"d"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if out[0].Tag != "dev" {
		t.Fatalf("tag default: %s", out[0].Tag)
	}
}

func TestParseEnvJSON_RejectInvalidTag(t *testing.T) {
	_, err := ParseEnvJSON(`[{"alias":"a","host":"h","port":5432,"user":"u","database":"d","tag":"yolo"}]`)
	if err == nil {
		t.Fatal("expected tag error")
	}
}

func TestParseEnvJSON_RequiredFields(t *testing.T) {
	cases := []string{
		`[{"host":"h","port":5432,"user":"u","database":"d"}]`,            // alias missing
		`[{"alias":"a","port":5432,"user":"u","database":"d"}]`,           // host missing
		`[{"alias":"a","host":"h","user":"u","database":"d"}]`,            // port missing (0)
		`[{"alias":"a","host":"h","port":99999,"user":"u","database":"d"}]`, // port out of range
		`[{"alias":"a","host":"h","port":5432,"database":"d"}]`,           // user missing
		`[{"alias":"a","host":"h","port":5432,"user":"u"}]`,               // database missing
	}
	for i, raw := range cases {
		if _, err := ParseEnvJSON(raw); err == nil {
			t.Fatalf("case %d: expected error for %s", i, raw)
		}
	}
}

func TestEnvKey_Deterministic(t *testing.T) {
	a := envKey("alias", "host", 5432, "db", "u")
	b := envKey("alias", "host", 5432, "db", "u")
	c := envKey("alias", "host", 5432, "other", "u")
	if a != b {
		t.Fatal("same input → different key")
	}
	if a == c {
		t.Fatal("different input → same key")
	}
	if len(a) != 16 {
		t.Fatalf("key length: %d", len(a))
	}
}
