package main

import (
	"studio/internal/sessionhost"
)

func (a *App) CanSteer(id string, atts []Attachment) bool { return a.turnIO().CanSteer(id, atts) }

func steerMarker(s *Session) string { return sessionhost.SteerMarker(s) }
