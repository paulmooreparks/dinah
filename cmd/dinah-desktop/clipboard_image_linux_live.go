//go:build live && linux && cgo

package main

// Helpers for clipboard_image_linux_live_test.go, which cannot use cgo
// itself because cgo is not allowed in test files. The live tag keeps them
// out of every ordinary build.

/*
#cgo pkg-config: gtk+-3.0
#include <string.h>
#include <gtk/gtk.h>

typedef struct {
	guchar *bytes;
	gsize size;
	int ok;
	int done;
	GMutex lock;
	GCond cond;
} dinahTestWrite;

// dinahTestWriteOnLoop runs on GTK's main loop. A GdkPixbufLoader for PNG
// (GdkPixbuf's documentation) decodes the bytes, and gtk_clipboard_set_image
// makes the picture the clipboard's contents, GTK answering requests for it
// from then on (GTK 3's GtkClipboard documentation).
static gboolean dinahTestWriteOnLoop(gpointer data) {
	dinahTestWrite *w = data;
	int ok = 0;
	GdkPixbufLoader *loader = gdk_pixbuf_loader_new_with_type("png", NULL);
	if (loader != NULL) {
		gboolean written = gdk_pixbuf_loader_write(loader, w->bytes, w->size, NULL);
		gboolean closed = gdk_pixbuf_loader_close(loader, NULL);
		GdkPixbuf *pixbuf = written && closed ? gdk_pixbuf_loader_get_pixbuf(loader) : NULL;
		if (pixbuf != NULL) {
			gtk_clipboard_set_image(gtk_clipboard_get(GDK_SELECTION_CLIPBOARD), pixbuf);
			ok = 1;
		}
		g_object_unref(loader);
	}
	g_mutex_lock(&w->lock);
	w->ok = ok;
	w->done = 1;
	g_cond_signal(&w->cond);
	g_mutex_unlock(&w->lock);
	return G_SOURCE_REMOVE;
}

// dinahTestWriteClipboard schedules the write with g_idle_add and waits up
// to five seconds for it with GLib's GMutex and GCond. On a timeout the
// state is left allocated, since the loop may still run the write later.
static int dinahTestWriteClipboard(const void *bytes, gsize size) {
	dinahTestWrite *w = g_new0(dinahTestWrite, 1);
	w->bytes = g_malloc(size);
	memcpy(w->bytes, bytes, size);
	w->size = size;
	g_mutex_init(&w->lock);
	g_cond_init(&w->cond);
	gint64 until = g_get_monotonic_time() + 5 * G_TIME_SPAN_SECOND;
	g_idle_add(dinahTestWriteOnLoop, w);
	g_mutex_lock(&w->lock);
	while (!w->done) {
		if (!g_cond_wait_until(&w->cond, &w->lock, until)) {
			g_mutex_unlock(&w->lock);
			return 0;
		}
	}
	int ok = w->ok;
	g_mutex_unlock(&w->lock);
	g_mutex_clear(&w->lock);
	g_cond_clear(&w->cond);
	g_free(w->bytes);
	g_free(w);
	return ok;
}

// dinahTestStartGTK initialises GTK on the calling thread, which becomes
// the thread GTK's functions must be called from. gtk_init_check returns
// FALSE instead of ending the program when no display can be opened.
static int dinahTestStartGTK(void) {
	return gtk_init_check(NULL, NULL) ? 1 : 0;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// startGTK initialises GTK on this goroutine's thread, which the caller has
// locked with runtime.LockOSThread, and says whether a display was opened.
func startGTK() bool { return C.dinahTestStartGTK() != 0 }

// runGTK runs GTK's main loop on the thread startGTK ran on, for good.
func runGTK() { C.gtk_main() }

// setClipboardPicture makes png, decoded by GdkPixbuf, the clipboard's
// picture. GTK's main loop must be running.
func setClipboardPicture(png []byte) error {
	if C.dinahTestWriteClipboard(unsafe.Pointer(&png[0]), C.gsize(len(png))) == 0 {
		return errors.New("the picture could not be put on the clipboard")
	}
	return nil
}
