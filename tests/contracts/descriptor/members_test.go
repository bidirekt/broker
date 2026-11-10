package descriptor_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func membersDocument(t *testing.T, source string) any {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(source), &document))

	return document
}

func TestMembers_EnumWithDistinctValuesIsValid(t *testing.T) {
	source := "type: enum\n" +
		"members: [available, pending, sold]\n" +
		"optional: true\n" +
		"nullable: true\n"

	violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

	assert.Empty(t, violations)
}

func TestMembers_ValuesAreDataSoAnyCharacterIsValid(t *testing.T) {
	source := "type: enum\n" +
		"members: [\"in.stock\", \"a;b\", \"(x)\", \"{y}\"]\n"

	violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

	assert.Empty(t, violations)
}

func TestMembers_EnumOrUnionWithoutMembersIsRequired(t *testing.T) {
	for _, schemaType := range []string{"enum", "union"} {
		for name, members := range map[string]string{
			"absent": "",
			"null":   "members: ~\n",
		} {
			t.Run(schemaType+" "+name, func(t *testing.T) {
				source := "type: object\n" +
					"properties:\n" +
					"  status:\n" +
					"    type: " + schemaType + "\n" +
					indent(members)

				violations := descriptor.Validate(descriptor.SchemaAt("schema"), membersDocument(t, source), "api.yaml")

				assert.Equal(t, []violation.Violation{{
					ErrorCode: "members.required",
					Path:      "properties;status",
					Source:    "api.yaml",
					Details:   map[string]string{"type": schemaType},
				}}, violations)
			})
		}
	}
}

func TestMembers_EmptyMembersOfAnEnumIsRejectedAtMembers(t *testing.T) {
	source := "type: object\n" +
		"properties:\n" +
		"  status:\n" +
		"    type: enum\n" +
		"    members: []\n"

	violations := descriptor.Validate(descriptor.SchemaAt("schema"), membersDocument(t, source), "api.yaml")

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "members.empty",
		Path:      "properties;status;members",
		Source:    "api.yaml",
		Details:   map[string]string{"type": "enum"},
	}}, violations)
}

func TestMembers_RepeatedValueIsRejectedAtItsIndex(t *testing.T) {
	source := "type: enum\n" +
		"members: [available, sold, sold]\n"

	violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "members.duplicate",
		Path:      "members;2",
		Source:    "api.yaml",
		Details:   map[string]string{"value": "sold"},
	}}, violations)
}

func TestMembers_ValueThatIsNotNonEmptyTextIsRejectedAtItsIndex(t *testing.T) {
	for name, value := range map[string]string{
		"empty text": `""`,
		"integer":    "3",
		"boolean":    "true",
		"null":       "~",
		"mapping":    "{a: b}",
	} {
		t.Run(name, func(t *testing.T) {
			source := "type: enum\n" +
				"members: [available, " + value + "]\n"

			violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

			assert.Equal(t, []violation.Violation{{
				ErrorCode: "members.not_text",
				Path:      "members;1",
				Source:    "api.yaml",
			}}, violations)
		})
	}
}

func TestMembers_ValueWithNulByteIsNotText(t *testing.T) {
	source := "type: enum\n" +
		"members: [\"a\\0b\"]\n"

	violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "members.not_text",
		Path:      "members;0",
		Source:    "api.yaml",
	}}, violations)
}

func TestMembers_EnumMembersThatIsNotAListIsAnInvalidKind(t *testing.T) {
	source := "type: enum\n" +
		"members: available\n"

	violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, "members", violations[0].Path)
	assert.Equal(t, map[string]string{"expected": "sequence", "got": "string"}, violations[0].Details)
}

func TestMembers_MembersOnAnotherTypeIsNotAllowed(t *testing.T) {
	source := "type: object\n" +
		"properties:\n" +
		"  name:\n" +
		"    type: string\n" +
		"    members: [a, b]\n"

	violations := descriptor.Validate(descriptor.SchemaAt("schema"), membersDocument(t, source), "api.yaml")

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "schema.key_not_allowed",
		Path:      "properties;name",
		Source:    "api.yaml",
		Details:   map[string]string{"key": "members", "type": "string"},
	}}, violations)
}

func TestMembers_MembersBesideRefIsNotAllowed(t *testing.T) {
	source := "ref: Status\n" +
		"members: [a, b]\n"

	violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "schema.key_not_allowed",
		Source:    "api.yaml",
		Details:   map[string]string{"key": "members", "type": "ref"},
	}}, violations)
}

func TestMembers_ShapeKeyOnAnEnumIsNotAllowed(t *testing.T) {
	source := "type: enum\n" +
		"members: [a]\n" +
		"items:\n" +
		"  type: string\n"

	violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "schema.key_not_allowed",
		Source:    "api.yaml",
		Details:   map[string]string{"key": "items", "type": "enum"},
	}}, violations)
}

func TestMembers_UnionOfRefsIsValid(t *testing.T) {
	source := "type: union\n" +
		"members:\n" +
		"  card: { ref: CardPayment }\n" +
		"  pix: { ref: PixPayment }\n" +
		"optional: true\n" +
		"nullable: true\n"

	violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

	assert.Empty(t, violations)
}

func TestMembers_EmptyMembersOfAUnionIsRejectedAtMembers(t *testing.T) {
	source := "type: union\n" +
		"members: {}\n"

	violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "members.empty",
		Path:      "members",
		Source:    "api.yaml",
		Details:   map[string]string{"type": "union"},
	}}, violations)
}

func TestMembers_UnionMemberWithoutRefIsRejectedAtItsKey(t *testing.T) {
	for name, member := range map[string]string{
		"null":      "~",
		"empty":     "{}",
		"empty ref": `{ ref: "" }`,
	} {
		t.Run(name, func(t *testing.T) {
			source := "type: union\n" +
				"members:\n" +
				"  card: " + member + "\n"

			violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

			assert.Equal(t, []violation.Violation{{
				ErrorCode: "member.ref_required",
				Path:      "members;card",
				Source:    "api.yaml",
				Details:   map[string]string{"member": "card"},
			}}, violations)
		})
	}
}

func TestMembers_UnionMemberWithAnotherKeyIsAnUnknownKey(t *testing.T) {
	source := "type: union\n" +
		"members:\n" +
		"  card: { ref: CardPayment, type: object }\n"

	violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

	assert.Equal(t, []violation.Violation{{
		ErrorCode: "key.unknown",
		Path:      "members;card;type",
		Source:    "api.yaml",
		Details:   map[string]string{"key": "type"},
	}}, violations)
}

func TestMembers_UnionMemberKeyWithAPathCharacterIsRejected(t *testing.T) {
	for _, key := range []string{"card(x)", "a)b", "card.x", "x;y"} {
		t.Run(key, func(t *testing.T) {
			source := "type: union\n" +
				"members:\n" +
				"  \"" + key + "\": { ref: CardPayment }\n"

			violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

			assert.Equal(t, []violation.Violation{{
				ErrorCode: "member.name_syntax",
				Path:      "members;" + key,
				Source:    "api.yaml",
				Details:   map[string]string{"key": key, "error": `must not contain ".", "[", "]", "{", "}", ";", "(" or ")"`},
			}}, violations)
		})
	}
}

func TestMembers_UnionMembersThatIsNotAMappingIsAnInvalidKind(t *testing.T) {
	source := "type: union\n" +
		"members: [card]\n"

	violations := descriptor.Validate(descriptor.Property(), membersDocument(t, source), "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "value.invalid_kind", violations[0].ErrorCode)
	assert.Equal(t, "members", violations[0].Path)
	assert.Equal(t, map[string]string{"expected": "mapping", "got": "sequence"}, violations[0].Details)
}

func indent(line string) string {
	if line == "" {
		return ""
	}

	return "    " + line
}
