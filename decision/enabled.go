package decision

import (
	"strings"

	"github.com/Zebbeni/protozoa/config"
)

// enabledCache memoises the filtered pools against the disabled set they were built from.
var enabledCache struct {
	key        string
	actions    []Action
	conditions []Condition
}

// disabledSet is the settings list as a lookup, plus the key the cache is stored under.
func disabledSet() (map[string]bool, string) {
	names := config.DisabledDecisionNodes()
	families := config.BasicOnlyConditionFamilies()
	key := strings.Join(names, "\x00") + "\x01" + strings.Join(families, "\x00")
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	// A basic-only family contributes every ADVANCED read below its root.
	for _, rootName := range families {
		for _, c := range MutableConditions {
			if Names[c] != rootName {
				continue
			}
			if !IsBasicCondition(c) {
				// A family is identified by its root, so naming a non-root says nothing.
				break
			}
			for _, adv := range AdvancedConditions(c) {
				set[Names[adv]] = true
			}
			break
		}
	}
	return set, key
}

// refreshEnabled rebuilds the filtered pools if the disabled set moved.
func refreshEnabled() {
	set, key := disabledSet()
	if enabledCache.key == key && enabledCache.actions != nil {
		return
	}
	actions := make([]Action, 0, len(MutableActions))
	for _, a := range MutableActions {
		if !set[Names[a]] {
			actions = append(actions, a)
		}
	}
	conditions := make([]Condition, 0, len(MutableConditions))
	for _, c := range MutableConditions {
		if !set[Names[c]] {
			conditions = append(conditions, c)
		}
	}
	// A pool emptied by the settings would panic in rng.Intn(0).
	if len(actions) == 0 {
		actions = append(actions, MutableActions...)
	}
	if len(conditions) == 0 {
		conditions = append(conditions, MutableConditions...)
	}
	enabledCache.key, enabledCache.actions, enabledCache.conditions = key, actions, conditions
}

func EnabledActions() []Action {
	refreshEnabled()
	return enabledCache.actions
}

func EnabledConditions() []Condition {
	refreshEnabled()
	return enabledCache.conditions
}

func IsNodeEnabled(v interface{}) bool {
	for _, a := range EnabledActions() {
		if a == v {
			return true
		}
	}
	for _, c := range EnabledConditions() {
		if c == v {
			return true
		}
	}
	return false
}

// MutableNodeTypes lists every node type the settings can toggle, in declaration order.
func MutableNodeTypes() []interface{} {
	out := make([]interface{}, 0, len(MutableActions)+len(MutableConditions))
	for _, a := range MutableActions {
		out = append(out, a)
	}
	for _, c := range MutableConditions {
		out = append(out, c)
	}
	return out
}
