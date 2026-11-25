package canon

import (
	"encoding/json"
	"testing"
)

// Vectors from RFC 8785 section 3.2.2 and appendix B.
func TestJCS(t *testing.T) {
	for in, want := range map[string]string{
		`{"numbers":[333333333.33333329,1E30,4.50,2e-3,0.000000000000000000000000001],"literals":[null,true,false]}`: `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27]}`,
		// Keys sort by UTF-16 code units: U+1F600 (surrogates D83D) sorts before U+FB33.
		`{"\u20ac":1,"\r":2,"1":3,"\ud83d\ude00":4,"\u0080":5,"\u00f6":6,"\ufb33":7}`: "{\"\\r\":2,\"1\":3,\"\u0080\":5,\"\u00f6\":6,\"\u20ac\":1,\"\U0001f600\":4,\"\ufb33\":7}",
		`[-0, 1e21, 1e-7, 123456789012345680000, 9007199254740992]`:                   `[0,1e+21,1e-7,123456789012345680000,9007199254740992]`,
		// Only quote, backslash and control characters are escaped; <&> and / are not.
		`"<&>/\u000f\n\"\\"`: `"<&>/\u000f\n\"\\"`,
	} {
		var v any
		if err := json.Unmarshal([]byte(in), &v); err != nil {
			t.Fatal(err)
		}
		got, err := jcs(v)
		if err != nil || string(got) != want {
			t.Errorf("jcs(%s)\n got %s %v\nwant %s", in, got, err, want)
		}
	}
}
