package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A customer must never see the internal group name or its multiplier.
func TestPublicGroupHidesInternalGroups(t *testing.T) {
	assert.Equal(t, "default", PublicUserGroup("plus"))
	assert.Equal(t, "default", PublicUserGroup("default"))
	assert.Equal(t, "vip", PublicUserGroup("vip"))

	assert.True(t, IsHiddenUserGroup("plus"))
	assert.False(t, IsHiddenUserGroup("default"))

	groups := PublicGroupMap(map[string]string{"default": "Default group", "plus": "User group"})
	assert.Equal(t, map[string]string{"default": "Default group"}, groups)

	ratios := PublicGroupRatioMap(map[string]float64{"default": 1, "plus": 3})
	assert.Equal(t, map[string]float64{"default": 1}, ratios)
}

// A customer on a hidden group may address it by its public name; the request
// still bills the internal group.
func TestResolvePublicGroupReachesHiddenGroup(t *testing.T) {
	assert.Equal(t, "plus", ResolvePublicGroup("default", "plus"), "public name must map back")
	assert.Equal(t, "plus", ResolvePublicGroup("plus", "plus"), "internal name still accepted")
	assert.Equal(t, "default", ResolvePublicGroup("default", "default"))
	assert.Equal(t, "", ResolvePublicGroup("", "plus"))
}
