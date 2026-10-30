package main

import "studio/internal/regional"

// HostLocale returns the language tag of the person's regional format on
// this computer, such as en-SG-u-hc-h23, so that the page writes dates and
// times as the person's computer writes them (regional.js). It returns ""
// when the format cannot be read, and the page then uses its own default.
func (a *App) HostLocale() string { return regional.HostLocale() }
