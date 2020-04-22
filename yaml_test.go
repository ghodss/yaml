package yaml

import (
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

type MarshalTest struct {
	A string
	B int64
	// Would like to test float64, but it's not supported in go-yaml.
	// (See https://github.com/go-yaml/yaml/issues/83.)
	C float32
}

func TestMarshal(t *testing.T) {
	f32String := strconv.FormatFloat(math.MaxFloat32, 'g', -1, 32)
	s := MarshalTest{"a", math.MaxInt64, math.MaxFloat32}
	e := []byte(fmt.Sprintf("A: a\nB: %d\nC: %s\n", int64(math.MaxInt64), f32String))

	y, err := Marshal(s)
	if err != nil {
		t.Errorf("error marshaling YAML: %v", err)
	}

	if !reflect.DeepEqual(y, e) {
		t.Errorf("marshal YAML was unsuccessful, expected: %#v, got: %#v",
			string(e), string(y))
	}
}

type UnmarshalPrimitives struct {
	Number int
	String string
	Bool   bool
}

type UnmarshalStringMap struct {
	Dict map[string]string
}

type UnmarshalNestedString struct {
	NestedString NestedString
}

type NestedString struct {
	String string
}

type UnmarshalSlice struct {
	Slice []NestedStrings
}

type NestedStrings struct {
	String    string
	StringPtr *string
}

func TestUnmarshal(t *testing.T) {
	y := []byte("string: \"1\"")
	s1 := UnmarshalPrimitives{}
	e1 := UnmarshalPrimitives{String: "1"}
	unmarshalEqual(t, y, &s1, &e1)

	y = []byte("bool: true")
	s1 = UnmarshalPrimitives{}
	e1 = UnmarshalPrimitives{Bool: true}
	unmarshalEqual(t, y, &s1, &e1)

	y = []byte("bool: true")
	s1 = UnmarshalPrimitives{}
	e1 = UnmarshalPrimitives{Bool: true}
	unmarshalEqual(t, y, &s1, &e1)

	y = []byte("nestedString:\n  string: hello")
	s2 := UnmarshalNestedString{}
	e2 := UnmarshalNestedString{NestedString{"hello"}}
	unmarshalEqual(t, y, &s2, &e2)

	y = []byte("slice:\n  - string: abc\n    stringPtr: def\n  - string: \"123\"\n    stringPtr: \"456\"\n")
	s3 := UnmarshalSlice{}
	e3 := UnmarshalSlice{[]NestedStrings{{"abc", strPtr("def")}, {"123", strPtr("456")}}}
	unmarshalEqual(t, y, &s3, &e3)

	y = []byte("dict:\n  b: balloon")
	s4 := UnmarshalStringMap{}
	e4 := UnmarshalStringMap{map[string]string{"b": "balloon"}}
	unmarshalEqual(t, y, &s4, &e4)

	y = []byte(`
a:
  name: TestA
b:
  name: TestB
`)
	type NamedThing struct {
		Name string `json:"name"`
	}
	s5 := map[string]*NamedThing{}
	e5 := map[string]*NamedThing{
		"a": {Name: "TestA"},
		"b": {Name: "TestB"},
	}
	unmarshalEqual(t, y, &s5, &e5)
}

// prettyFunctionName converts a slice of JSONOpt function pointers to a human
// readable string representation.
func prettyFunctionName(opts []JSONOpt) []string {
	var r []string
	for _, o := range opts {
		r = append(r, runtime.FuncForPC(reflect.ValueOf(o).Pointer()).Name())
	}
	return r
}

func unmarshalEqual(t *testing.T, y []byte, s, e interface{}, opts ...JSONOpt) {
	t.Helper()
	err := Unmarshal(y, s, opts...)
	if err != nil {
		t.Errorf("Unmarshal(%#q, s, %v) = %v", string(y), prettyFunctionName(opts), err)
		return
	}

	if !reflect.DeepEqual(s, e) {
		t.Errorf("Unmarshal(%#q, s, %v) = %+#v; want %+#v", string(y), prettyFunctionName(opts), s, e)
	}
}

// TestUnmarshalErrors tests that we return an error on ambiguous YAML.
func TestUnmarshalErrors(t *testing.T) {
	for _, tc := range []struct {
		yaml    []byte
		want    UnmarshalPrimitives
		wantErr string
	}{
		{
			yaml: []byte("number: 1"),
			want: UnmarshalPrimitives{Number: 1},
		},
		{
			// Order does not matter.
			yaml: []byte("bool: true\nnumber: 2"),
			want: UnmarshalPrimitives{Number: 2, Bool: true},
		},
		{
			// By default, unknown field is ignored.
			yaml: []byte("string: foo\nunknownField: 2"),
			want: UnmarshalPrimitives{String: "foo"},
		},
		{
			// Declaring `a` twice produces an error.
			yaml:    []byte("number: 1\nnumber: 2"),
			wantErr: `mapping key "number" already defined at line 1`,
		},
		{
			// Not ignoring first declaration of String with wrong type.
			yaml:    []byte("a: [1,2,3]\na: value-of-a"),
			wantErr: `mapping key "a" already defined at line 1`,
		},
		{
			// Declaring field `bool` twice.
			yaml:    []byte("bool: true\nbool: false"),
			wantErr: `mapping key "bool" already defined at line 1`,
		},
	} {
		s := UnmarshalPrimitives{}
		err := Unmarshal(tc.yaml, &s)
		if tc.wantErr != "" && err == nil {
			t.Errorf("UnmarshalStrict(%#q, &s) = nil; want error", string(tc.yaml))
			continue
		}
		if tc.wantErr == "" && err != nil {
			t.Errorf("UnmarshalStrict(%#q, &s) = %v; want no error", string(tc.yaml), err)
			continue
		}
		// We only expect errors during unmarshalling YAML.
		if want := "yaml: unmarshal errors"; tc.wantErr != "" && !strings.Contains(err.Error(), want) {
			t.Errorf("UnmarshalStrict(%#q, &s) = %v; want err contains %#q", string(tc.yaml), err, want)
		}
		if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("UnmarshalStrict(%#q, &s) = %v; want err contains %#q", string(tc.yaml), err, tc.wantErr)
		}

		// Even if there was an error, we continue the test: We expect that all
		// errors occur during YAML unmarshalling. Such errors leaves `s` unmodified
		// and the following check will compare default values of `UnmarshalPrimitives`.

		if !reflect.DeepEqual(s, tc.want) {
			t.Errorf("UnmarshalStrict(%#q, &s) = %+#v; want %+#v", string(tc.yaml), s, tc.want)
		}
	}
}

type Case struct {
	input  string
	output string
	// By default we test that reversing the output == input. But if there is a
	// difference in the reversed output, you can optionally specify it here.
	reverse *string
}

type RunType int

const (
	RunTypeJSONToYAML RunType = iota
	RunTypeYAMLToJSON
)

func TestJSONToYAML(t *testing.T) {
	cases := []Case{
		{
			`{"t":"a"}`,
			"t: a\n",
			nil,
		}, {
			`{"t":null}`,
			"t: null\n",
			nil,
		},
	}

	runCases(t, RunTypeJSONToYAML, cases)
}

func TestYAMLToJSON(t *testing.T) {
	cases := []Case{
		{
			"t: a\n",
			`{"t":"a"}`,
			nil,
		}, {
			"t: \n",
			`{"t":null}`,
			strPtr("t: null\n"),
		}, {
			"t: null\n",
			`{"t":null}`,
			nil,
		}, {
			"1: a\n",
			`{"1":"a"}`,
			strPtr("\"1\": a\n"),
		}, {
			"1000000000000000000000000000000000000: a\n",
			`{"1e+36":"a"}`,
			strPtr("\"1e+36\": a\n"),
		}, {
			"1e+36: a\n",
			`{"1e+36":"a"}`,
			strPtr("\"1e+36\": a\n"),
		}, {
			"\"1e+36\": a\n",
			`{"1e+36":"a"}`,
			nil,
		}, {
			"\"1.2\": a\n",
			`{"1.2":"a"}`,
			nil,
		}, {
			"- t: a\n",
			`[{"t":"a"}]`,
			nil,
		}, {
			"- t: a\n" +
				"- t:\n" +
				"      b: 1\n" +
				"      c: 2\n",
			`[{"t":"a"},{"t":{"b":1,"c":2}}]`,
			nil,
		}, {
			`[{t: a}, {t: {b: 1, c: 2}}]`,
			`[{"t":"a"},{"t":{"b":1,"c":2}}]`,
			strPtr("- t: a\n" +
				"- t:\n" +
				"      b: 1\n" +
				"      c: 2\n"),
		}, {
			"- t: \n",
			`[{"t":null}]`,
			strPtr("- t: null\n"),
		}, {
			"- t: null\n",
			`[{"t":null}]`,
			nil,
		},
	}

	// Cases that should produce errors.
	_ = []Case{
		{
			"~: a",
			`{"null":"a"}`,
			nil,
		}, {
			"a: !!binary gIGC\n",
			"{\"a\":\"\x80\x81\x82\"}",
			nil,
		},
	}

	runCases(t, RunTypeYAMLToJSON, cases)
}

func runCases(t *testing.T, runType RunType, cases []Case) {
	var f func([]byte) ([]byte, error)
	var invF func([]byte) ([]byte, error)
	var msg string
	var invMsg string
	if runType == RunTypeJSONToYAML {
		f = JSONToYAML
		invF = YAMLToJSON
		msg = "JSON to YAML"
		invMsg = "YAML back to JSON"
	} else {
		f = YAMLToJSON
		invF = JSONToYAML
		msg = "YAML to JSON"
		invMsg = "JSON back to YAML"
	}

	for _, c := range cases {
		// Convert the string.
		t.Logf("converting %s\n", c.input)
		output, err := f([]byte(c.input))
		if err != nil {
			t.Errorf("Failed to convert %s, input: `%s`, err: %v", msg, c.input, err)
		}

		// Check it against the expected output.
		if string(output) != c.output {
			t.Errorf("Failed to convert %s, input: `%s`, expected `%s`, got `%s`",
				msg, c.input, c.output, string(output))
		}

		// Set the string that we will compare the reversed output to.
		reverse := c.input
		// If a special reverse string was specified, use that instead.
		if c.reverse != nil {
			reverse = *c.reverse
		}

		// Reverse the output.
		input, err := invF(output)
		if err != nil {
			t.Errorf("Failed to convert %s, input: `%s`, err: %v", invMsg, string(output), err)
		}

		// Check the reverse is equal to the input (or to *c.reverse).
		if string(input) != reverse {
			t.Errorf("Failed to convert %s, input: `%s`, expected `%s`, got `%s`",
				invMsg, string(output), reverse, string(input))
		}
	}
}

// To be able to easily fill in the *Case.reverse string above.
func strPtr(s string) *string {
	return &s
}

func TestYAMLToJSONDuplicateFields(t *testing.T) {
	const data = `
foo: bar
foo: baz
`
	if _, err := YAMLToJSON([]byte(data)); err == nil {
		t.Error("expected YAMLtoJSONStrict to fail on duplicate field names")
	}
}
