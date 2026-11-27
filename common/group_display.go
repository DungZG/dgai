package common

// hiddenUserGroups are internal routing groups used to apply a private
// multiplier to a subscription. Customers must never see them: every
// customer-facing surface reports such a user as the public default group.
//
// shortcut: the list is a constant; move it to an option if more internal
// groups are introduced.
var hiddenUserGroups = map[string]struct{}{
	"plus": {},
}

// PublicUserGroup maps an internal group to the name a customer may see.
func PublicUserGroup(group string) string {
	if IsHiddenUserGroup(group) {
		return "default"
	}
	return group
}

// IsHiddenUserGroup reports whether a group is an internal routing detail.
func IsHiddenUserGroup(group string) bool {
	_, hidden := hiddenUserGroups[group]
	return hidden
}

// PublicGroupMap renames hidden groups to their public name. A caller that
// already has an entry under the public name wins.
func PublicGroupMap(groups map[string]string) map[string]string {
	result := make(map[string]string, len(groups))
	for name, desc := range groups {
		if IsHiddenUserGroup(name) {
			continue
		}
		result[name] = desc
	}
	if _, ok := result["default"]; !ok {
		if desc, hidden := groups["default"]; hidden {
			result["default"] = desc
		}
	}
	return result
}

// PublicGroupRatioMap drops hidden groups from a ratio map. The public entry is
// left untouched: the customer's real multiplier is applied when billing.
func PublicGroupRatioMap(ratios map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(ratios))
	for name, ratio := range ratios {
		if IsHiddenUserGroup(name) {
			continue
		}
		result[name] = ratio
	}
	return result
}

// ResolvePublicGroup turns a group name the customer typed into the internal
// group that actually serves the request. A hidden group is reachable through
// its public name, so a customer paying the hidden multiplier can still create
// a token on the group the catalogue shows.
func ResolvePublicGroup(name string, userGroup string) string {
	if !IsHiddenUserGroup(userGroup) || name != PublicUserGroup(userGroup) {
		return name
	}
	return userGroup
}

// PublicGroupList rewrites a model's serving groups for the catalogue, keeping
// the order and removing duplicates created by the rename.
func PublicGroupList(groups []string) []string {
	if len(groups) == 0 {
		return groups
	}
	result := make([]string, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, name := range groups {
		visible := PublicUserGroup(name)
		if _, duplicate := seen[visible]; duplicate {
			continue
		}
		seen[visible] = struct{}{}
		result = append(result, visible)
	}
	return result
}
