package main

import "studio/internal/shortcodes"

/* The shortcodes a build adds to the Markdown dialect reach the content
   applets through the host's shortcodes service (content-host.js), which
   asks these two methods (docs/design/shortcodes.md). Dinah's own build
   registers none, and the service then says so. */

// ShortcodeCatalog lists the shortcodes this build adds, for completion,
// the Insert menu and forms.
func (a *App) ShortcodeCatalog() []shortcodes.Descriptor { return shortcodes.Catalog() }

// RenderShortcode draws one occurrence of a shortcode this build adds, as
// HTML the applet sanitises and shows in place; a container's HTML holds
// one <slot name="content"></slot> for the body the applet draws itself.
func (a *App) RenderShortcode(r shortcodes.Request) (string, error) { return shortcodes.Render(r) }
