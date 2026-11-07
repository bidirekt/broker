package breakreport

import "strings"

type SegmentKind int

const (
	PlainSegment SegmentKind = iota
	NameSegment
	CodeSegment
)

type Segment struct {
	Text string
	Kind SegmentKind
}

func (this Segment) IsName() bool {
	return this.Kind == NameSegment
}

func (this Segment) IsCode() bool {
	return this.Kind == CodeSegment
}

type Phrase []Segment

func (this Phrase) String() string {
	var text strings.Builder
	for _, segment := range this {
		text.WriteString(segment.Text)
	}
	return text.String()
}

func plain(text string) Segment {
	return Segment{Text: text, Kind: PlainSegment}
}

func name(text string) Segment {
	return Segment{Text: text, Kind: NameSegment}
}

func code(text string) Segment {
	return Segment{Text: text, Kind: CodeSegment}
}
