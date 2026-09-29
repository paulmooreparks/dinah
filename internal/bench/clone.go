package bench

// A resident snapshot hands one memoised value to every request, and reads
// change what they are handed, so every value Derive returns is cloned before
// it leaves this package. A clone shares nothing its holder may change with
// the value it was taken from: every slice, map and pointer it carries is its
// own, and its header is a copy-on-write Clone. TestEntityClonesShareNothingMutable
// holds each type's fields to that, so a field added later that is a slice, a
// map or a pointer fails it until Clone covers it.

// Clone answers a card its holder may change without changing c.
func (c *Card) Clone() *Card {
	if c == nil {
		return nil
	}
	clone := *c
	if c.FM != nil {
		clone.FM = c.FM.Clone()
	}
	if c.Links != nil {
		clone.Links = append([]Link(nil), c.Links...)
	}
	if c.Workstreams != nil {
		clone.Workstreams = append([]string(nil), c.Workstreams...)
	}
	if c.ColumnTiers != nil {
		clone.ColumnTiers = append([]ColumnTier(nil), c.ColumnTiers...)
	}
	return &clone
}

// Clone answers an item its holder may change without changing i.
func (i *Item) Clone() *Item {
	if i == nil {
		return nil
	}
	clone := *i
	if i.Citations != nil {
		clone.Citations = append([]Citation(nil), i.Citations...)
	}
	if i.Redacted != nil {
		redacted := *i.Redacted
		clone.Redacted = &redacted
	}
	if i.keys != nil {
		clone.keys = append([]string(nil), i.keys...)
	}
	return &clone
}

// Clone answers a comment its holder may change without changing c.
func (c *Comment) Clone() *Comment {
	if c == nil {
		return nil
	}
	clone := *c
	if c.Redacted != nil {
		redacted := *c.Redacted
		clone.Redacted = &redacted
	}
	return &clone
}

// Clone answers an attachment its holder may change without changing a.
func (a *Attachment) Clone() *Attachment {
	if a == nil {
		return nil
	}
	clone := *a
	return &clone
}

// cloneEvents answers a copy of a journal's events that shares no slice with
// them.
func cloneEvents(events []Event) []Event {
	if events == nil {
		return nil
	}
	clone := append([]Event(nil), events...)
	for n := range clone {
		if clone[n].Cards != nil {
			clone[n].Cards = append([]string(nil), clone[n].Cards...)
		}
		if clone[n].Citations != nil {
			clone[n].Citations = append([]CitationRecord(nil), clone[n].Citations...)
		}
		if clone[n].Accepted != nil {
			clone[n].Accepted = append([]string(nil), clone[n].Accepted...)
		}
		if clone[n].WrittenDuringRun != nil {
			clone[n].WrittenDuringRun = append([]string(nil), clone[n].WrittenDuringRun...)
		}
		if clone[n].Fields != nil {
			fields := make(map[string]string, len(clone[n].Fields))
			for key, value := range clone[n].Fields {
				fields[key] = value
			}
			clone[n].Fields = fields
		}
	}
	return clone
}
