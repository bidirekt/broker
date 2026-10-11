package contract_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/stretchr/testify/assert"
)

func TestRefTrail_Empty_ContainsNothing(t *testing.T) {
	trail := contract.RefTrail{}

	assert.False(t, trail.Contains("Comment"))
	assert.Empty(t, trail.Names())
}

func TestRefTrail_Enter_AddsTheNameAtTheEnd(t *testing.T) {
	trail := contract.RefTrail{}.Enter("Comment").Enter("Reply")

	assert.True(t, trail.Contains("Comment"))
	assert.True(t, trail.Contains("Reply"))
	assert.False(t, trail.Contains("Feed"))
	assert.Equal(t, []string{"Comment", "Reply"}, trail.Names())
}

func TestRefTrail_Enter_LeavesTheOriginalTrailUntouched(t *testing.T) {
	post := contract.RefTrail{}.Enter("Post")

	author := post.Enter("Author")
	editor := post.Enter("Editor")

	assert.Equal(t, []string{"Post"}, post.Names())
	assert.False(t, post.Contains("Author"))
	assert.Equal(t, []string{"Post", "Author"}, author.Names())
	assert.Equal(t, []string{"Post", "Editor"}, editor.Names())
}

func TestRefTrail_EnterOnSiblingBranches_DoesNotShareTheBackingArray(t *testing.T) {
	trunk := contract.RefTrail{}.Enter("Feed").Enter("Post").Enter("Author")

	first := trunk.Enter("Details")
	second := trunk.Enter("Avatar")

	assert.Equal(t, []string{"Feed", "Post", "Author", "Details"}, first.Names())
	assert.Equal(t, []string{"Feed", "Post", "Author", "Avatar"}, second.Names())
}
