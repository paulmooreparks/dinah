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
	return &clone
}

// Clone answers a comment its holder may change without changing c.
func (c *Comment) Clone() *Comment {
	if c == nil {
		return nil
	}
	clone := *c
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
	}
	return clone
}
