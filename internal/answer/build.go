package answer

import (
	"dinah/internal/verb"
)

// Build builds a library request for one command from a map of named
// arguments, reading the same parameter list the cli head composes its
// syntax from. A valued parameter's argument is a string and a marker's is a
// boolean, which is what the MCP schema publishes; an argument of any other
// type is read as the zero value. Identity (the actor, the basis and the four
// declared facts) is left to each head, because the heads read it from
// different places.
func Build(command string, arguments map[string]any) *verb.Request {
	req := &verb.Request{Verb: command}
	for _, param := range verb.Params(command) {
		// A parameter the table binds to no request field is one no verb
		// reads, so it is dropped here rather than landing on whatever field
		// shares its name. workbenches names its depth the same way the
		// root-scoped reads do and answers it without a request at all, and
		// assigning it anyway made the declaration and the code disagree
		// about a value nothing goes on to read.
		if param.Field == "" {
			continue
		}
		value, ok := arguments[param.Name]
		if !ok {
			continue
		}
		if param.Marker {
			flag, _ := value.(bool)
			assignMarker(req, param.Name, flag)
			continue
		}
		text, _ := value.(string)
		assignValue(req, param.Name, param.Field, text)
	}
	return req
}

// assignValue puts one named string argument on the request. The declared
// field rides beside the name because one word is not always one field: show
// and changes both take an argument spelled since, and the two mean different
// things, so the table is what says which field the value lands on rather
// than the spelling.
func assignValue(req *verb.Request, name, field, value string) {
	switch name {
	case "card":
		req.Card = value
	case "ref":
		req.Ref = value
	// The five checklist verbs beyond file name their target "item" rather
	// than "ref", because a caller composing one is looking at a checklist
	// and the sentence beside the argument says so. It lands on the same
	// field, since the reference grammar that resolves it is the same one.
	case "item":
		req.Ref = value
	// accept-divergence names its target "comment" rather than "ref",
	// because it ratifies a comment and nothing else, and the sentence
	// beside the argument says so. It lands on the same field for the same
	// reason "item" does.
	case "comment":
		req.Ref = value
	case "scheme":
		req.Scheme = value
	case "target":
		req.CiteTarget = value
	// link and unlink name their target "to" rather than "target", because
	// the two mean different things and land on different fields: a link's
	// target is a card reference the library resolves, and a citation's is an
	// opaque string nothing resolves.
	case "to":
		req.LinkTo = value
	case "observed":
		req.Observed = value
	case "expect-digest":
		req.ExpectedDigest = value
	case "note":
		req.Note = value
	// The three terminal checklist verbs name their answer "designation"
	// rather than "note", because what it carries is a reference to a
	// comment of the item rather than prose. It lands on the same field the
	// note landed on, which is the slot those verbs read their answer from.
	case "designation":
		req.Note = value
	case "owner":
		req.Owner = value
	case "column":
		req.Column = value
	case "since":
		// changes takes the opaque cursor a checkpoint handed back and show
		// takes a comment's one-based ordinal. Neither field may hold the
		// other's value, so the split is read off the declaration.
		if field == "SinceComment" {
			req.SinceComment = value
			break
		}
		req.Since = value
	case "query":
		req.Query = value
	case "group-by":
		req.GroupBy = value
	case "view":
		req.View = value
	case "depth":
		req.Depth = value
	case "root":
		req.Root = value
	case "max-depth":
		req.MaxDepth = value
	case "action":
		req.Action = value
	case "field":
		req.Field = value
	case "fields":
		req.Fields = value
	case "workstream":
		req.Workstream = value
	case "permission":
		req.Permission = value
	case "slug":
		req.Slug = value
	case "value":
		req.Value = value
	case "name":
		req.Value = value
	case "severity":
		req.Severity = value
	case "priority":
		req.Priority = value
	case "route":
		req.Route = value
	case "start-after":
		req.StartAfter = value
	case "start-by":
		req.StartBy = value
	case "due":
		req.Due = value
	case "tier":
		req.Tier = value
	case "at":
		req.At = value
	case "title":
		req.Title = value
	case "text":
		req.Text = value
	case "phrase":
		req.SearchText = value
	case "reason":
		req.Reason = value
	case "kind":
		req.Kind = value
	case "state":
		req.State = value
	case "capacity":
		req.Capacity = value
	case "before":
		req.Before = value
	case "file":
		req.File = value
	case "remint":
		req.Remint = value
	case "expires":
		if parsed, err := verb.ParseDuration(value); err == nil {
			req.Expires = parsed
		}
	case "timeout":
		// Both machine heads hold this argument of changes back
		// (internal/mcp's argumentExemptions, dinah-546, and
		// internal/httphead's paramExemptions), so each refuses a call
		// naming it before dispatch ever reaches here; the case exists for
		// the same reason check's own held-back markers below carry one:
		// the request-building plumbing every declared parameter gets is
		// declared once, independent of which head publishes the name.
		if parsed, err := verb.ParseDuration(value); err == nil {
			req.Timeout = parsed
		}
	}
}

// assignMarker puts one named flag on the request.
func assignMarker(req *verb.Request, name string, value bool) {
	switch name {
	case "override":
		req.Override = value
	case "replace":
		req.Replace = value
	case "yes":
		req.Confirm = value
	case "force":
		req.Force = value
	case "ready":
		req.ReadyOnly = value
	case "unresolved":
		req.Unresolved = value
	case "all":
		req.All = value
	case "finish":
		req.Finish = value
	case "migrate-ordinals":
		req.MigrateOrdinals = value
	case "migrate-slugs":
		req.MigrateSlugs = value
	case "migrate-columns":
		req.MigrateColumns = value
	case "migrate-vocabulary":
		req.MigrateVocabulary = value
	case "migrate-container":
		req.MigrateContainer = value
	case "migrate-branches":
		req.MigrateBranches = value
	case "migrate-newlines":
		req.MigrateNewlines = value
	case "file-standing":
		req.FileStanding = value
	case "migrate-numbers":
		req.MigrateNumbers = value
	case "migrate-designations":
		req.MigrateDesignations = value
	case "migrate-applies-when":
		req.MigrateAppliesWhen = value
	case "migrate-schedule":
		req.MigrateSchedule = value
	case "migrate-holds":
		req.MigrateHolds = value
	case "migrate-raw-lines":
		req.MigrateRawLines = value
	case "rehearse":
		req.Rehearse = value
	case "force-claims":
		req.ForceClaims = value
	case "migrate-workstreams":
		req.MigrateWorkstreams = value
	case "witness":
		req.MigrateWitness = value
	case "renumber":
		req.Renumber = value
	case "no-claim":
		req.NoClaim = value
	case "full-pending":
		req.FullPending = value
	case "brief":
		req.Brief = value
	case "explain":
		req.Explain = value
	case "archived":
		req.Archived = value
	case "wait":
		// Both machine heads hold this argument of changes back, so each
		// refuses a call naming it before dispatch ever reaches here; see
		// the case for "timeout" in assignValue above.
		req.Wait = value
	case "plain":
		// Both machine heads hold this argument of view back, for the
		// reason "wait" gives.
		req.ViewPlain = value
	case "watch":
		req.ViewWatch = value
	}
}
