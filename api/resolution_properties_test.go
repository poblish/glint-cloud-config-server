package api

import (
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func Test_resolvePlaceholders(t *testing.T) {
	tests := []placeholdersTest{
		{
			name: "multi-level-recurse",
			inputs: map[string]interface{}{
				"b.d.a": "  ${c.d.a}${}",
				"b.a.a": "${c.d.a}",
				"a.a.a": "apple",
				"c.d.a": "${a.d.a}",
				"a.d.a": "${a.b.a}",
				"a.a.b": "bye",
				"a.c.c": "${greeting:Hey} ${a.a.c}  ",
				"a.d.b": "${a.b.d}",
				"a.b.d": "${a.b.a}",
				"a.e.b": "${missing}  ${a.a.b}",
				"a.d.c": "${a.c.c}",
				"a.a.c": "cat",
				"b.d.b": "${a.a.b}",
				"a.b.a": "${a.a.a}  ",
				"a.b.c": "${a.d.c}",
				"c.e.a": "  ${c.d.a}  ",
			},
			expectation: map[string]interface{}{
				"a.a.a": "apple",
				"a.a.b": "bye",
				"a.a.c": "cat",
				"a.b.a": "apple  ",
				"a.b.c": "Hey cat  ",
				"a.b.d": "apple  ",
				"a.c.c": "Hey cat  ",
				"a.d.a": "apple  ",
				"a.d.b": "apple  ",
				"a.d.c": "Hey cat  ",
				"a.e.b": "  bye",
				"b.a.a": "apple  ",
				"b.d.a": "  apple  ",
				"b.d.b": "bye",
				"c.d.a": "apple  ",
				"c.e.a": "  apple    ",
			},
			messages: []string{
				"Missing value for property [missing]",
				"Missing placeholder [${}] for property [b.d.a]",
			},
		},
	}
	for _, tt := range tests {
		for i := 1; i <= 10; i++ {
			t.Run(tt.name, func(t *testing.T) {
				// Resolution is destructive, so let's make a *deep* copy
				newData := map[string]interface{}{}
				e := deepCopyViaJSON(tt.inputs, newData)
				assert.NoError(t, e)

				rr := PropertiesResolver{data: newData}
				result := rr.resolvePlaceholdersFromTop()
				assert.Equal(t, tt.expectation, result)
				assert.ElementsMatch(t, tt.messages, rr.messages)
			})
		}
	}
}

type placeholdersTest struct {
	name        string
	inputs      ResolvedConfigValues
	expectation ResolvedConfigValues
	messages    []string
}

func MapCopy(dst, src interface{}) {
	dv, sv := reflect.ValueOf(dst), reflect.ValueOf(src)

	for _, k := range sv.MapKeys() {
		dv.SetMapIndex(k, sv.MapIndex(k))
	}
}

// Must deal with floats rather than ints if we're going to use this approach
func deepCopyViaJSON(src map[string]interface{}, dest map[string]interface{}) error {
	if src == nil {
		return errors.New("src is nil. You cannot read from a nil map")
	}
	if dest == nil {
		return errors.New("dest is nil. You cannot insert to a nil map")
	}
	jsonStr, err := json.Marshal(src)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonStr, &dest)
	if err != nil {
		return err
	}
	return nil
}
