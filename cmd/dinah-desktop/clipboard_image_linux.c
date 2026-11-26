//go:build linux && cgo

// The C half of clipboard_image_linux.go. It is in its own file because
// that file exports a Go function, and cgo then allows no C definitions in
// its preamble (the cgo command's documentation, "C references to Go").

#include <gtk/gtk.h>
#include "_cgo_export.h"

// dinahImageReceived is the GtkClipboardImageReceivedFunc of the request.
// GTK 3 documents pixbuf as the received image, or NULL when the clipboard
// holds none or it could not be converted; GTK keeps ownership of it.
// gdk_pixbuf_save_to_buffer (GdkPixbuf's documentation) writes it as PNG
// into a new buffer, which is freed with g_free, and on failure sets a
// GError, freed with g_error_free.
static void dinahImageReceived(GtkClipboard *clipboard, GdkPixbuf *pixbuf, gpointer data) {
	uintptr_t handle = (uintptr_t)data;
	(void)clipboard;
	if (pixbuf == NULL) {
		dinahClipboardImageDone(handle, NULL, 0, NULL);
		return;
	}
	gchar *buffer = NULL;
	gsize size = 0;
	GError *error = NULL;
	if (!gdk_pixbuf_save_to_buffer(pixbuf, &buffer, &size, "png", &error, NULL)) {
		dinahClipboardImageDone(handle, NULL, 0, error != NULL ? error->message : (char *)"no reason given");
		if (error != NULL) g_error_free(error);
		g_free(buffer);
		return;
	}
	dinahClipboardImageDone(handle, (guchar *)buffer, size, NULL);
	g_free(buffer);
}

// dinahRequestOnLoop runs on GTK's main loop. gtk_clipboard_get with
// GDK_SELECTION_CLIPBOARD is the clipboard of Copy and Paste, and
// gtk_clipboard_request_image asks it for an image and calls the callback
// once with the result (GTK 3's GtkClipboard documentation). Returning
// G_SOURCE_REMOVE makes the idle source run once.
static gboolean dinahRequestOnLoop(gpointer data) {
	gtk_clipboard_request_image(gtk_clipboard_get(GDK_SELECTION_CLIPBOARD), dinahImageReceived, data);
	return G_SOURCE_REMOVE;
}

// dinahRequestClipboardImage is called from any Go thread. GTK's functions
// may be called only from the thread that runs its main loop, so the
// request is scheduled there with g_idle_add, which GLib's documentation
// ("The Main Event Loop") says can be called from any thread; the source is
// added to the default main context, which gtk_main runs.
void dinahRequestClipboardImage(uintptr_t handle) {
	g_idle_add(dinahRequestOnLoop, (gpointer)handle);
}
