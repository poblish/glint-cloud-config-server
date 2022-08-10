package api

import (
	"fmt"
	"github.com/Masterminds/sprig"
	"github.com/rs/zerolog/log"
	"regexp"
	"strings"
	"text/template"
)

const UnresolvedPropertyResult = ""

type PropertiesResolvable interface {
	resolvePlaceholdersFromTop() (ResolvedConfigValues, error)
}

type PropertiesResolver struct {
	data     ResolvedConfigValues
	error    error
	messages []string
}

var placeholderRegex = regexp.MustCompile(`\${([^}]*)}`)

func (pr *PropertiesResolver) resolvePlaceholdersFromTop() (ResolvedConfigValues, error) {
	return pr.resolvePlaceholders(pr.data)
}

func (pr *PropertiesResolver) resolvePlaceholders(currentMap map[string]interface{}) (ResolvedConfigValues, error) {
	for propertyName, v := range currentMap {
		switch typedVal := v.(type) {
		case map[string]interface{}:
			_, _ = pr.resolvePlaceholders(typedVal)
		case []interface{}:
			resolved := make([]string, len(typedVal))
			stack := newStack()
			for i, eachUnresolved := range typedVal {
				resolved[i] = pr.resolveString(currentMap, propertyName, eachUnresolved.(string), stack)
			}
			currentMap[propertyName] = resolved // replace the whole thing
		case string:
			currentMap[propertyName] = pr.resolveString(currentMap, propertyName, typedVal, newStack())
		}
	}
	return pr.data, pr.error
}

var Template = template.New("").
	Funcs(sprig.TxtFuncMap())

// TODO Should missing properties be a configurable fatal error?
func (pr *PropertiesResolver) resolveString(currentMap map[string]interface{}, propertyName string, value string, stack map[string]interface{}) string {
	goTemplatesResult := value

	// Look for possible Go templates
	if strings.Contains(value, "{{") && strings.Contains(value, "}}") {
		var buf strings.Builder
		tmpl, e := Template.Parse(value)
		if e != nil {
			pr.error = e
			return ""
		}

		tmpl.Delims("{{", "}}")

		data := map[string]interface{}{
			"Applications": []string{"accounts", "application"},
			"Profiles":     []string{"prod-uk", "prod", "base"},
		}

		if err := tmpl.Execute(&buf, data); err != nil {
			pr.error = e
			return ""
		}

		goTemplatesResult = buf.String()
	}

	if pr.error != nil {
		return ""
	}

	propertiesResult := placeholderRegex.ReplaceAllStringFunc(goTemplatesResult, func(foundMatch string) string {
		sourcePropertyWithDefault := pr.getPropertyClauseFromMatch(foundMatch)
		if sourcePropertyWithDefault[0] == "" {
			// ${} is not acceptable
			pr.addMessage("Missing placeholder [%s] for property [%s]", foundMatch, propertyName)
			return UnresolvedPropertyResult
		}

		if currVal, ok := pr.resolvePropertyName(sourcePropertyWithDefault[0]); ok {
			switch currValStr := currVal.(type) {
			case string:
				if strings.Contains(currValStr, "${") {
					// recurse to resolve placeholder...
					propName := sourcePropertyWithDefault[0]

					///////////// Handle stack overflows
					if stack != nil && stack[propName] != nil {
						pr.error = fmt.Errorf("stack overflow found when resolving ${%s}", propName)
						return ""
					}
					stack[propName] = true
					/////////////

					currentMap[propName] = pr.resolveString(currentMap, propName, currValStr, stack)
				} else {
					// this value is fine
					return currValStr
				}
			default:
				// this value is fine, but convert to a string
				return fmt.Sprintf("%v", currVal)
			}
		}

		// Re-check post recurse
		if updatedPropertyValue, ok := pr.resolvePropertyName(sourcePropertyWithDefault[0]); ok {
			return updatedPropertyValue.(string)
		}

		// Not found, do we have a default value?
		if len(sourcePropertyWithDefault) < 2 {
			// No match, no default
			pr.addMessage("Missing value for property [%s]", sourcePropertyWithDefault[0])
		} else if len(sourcePropertyWithDefault) > 1 {
			// No match, use available default
			return sourcePropertyWithDefault[1]
		}

		return UnresolvedPropertyResult
	})

	return propertiesResult
}

func (pr *PropertiesResolver) resolvePropertyName(name string) (interface{}, bool) {
	val, ok := pr.data[name]
	return val, ok
}

func (pr *PropertiesResolver) getPropertyClauseFromMatch(match string) []string {
	return strings.Split(strings.TrimSpace(match[2:len(match)-1]), ":")
}

func (pr *PropertiesResolver) addMessage(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	pr.messages = append(pr.messages, msg)
	log.Warn().Msg(msg)
}

func newStack() map[string]interface{} {
	return map[string]interface{}{}
}
