//go:build linux

package main

/*
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>
#include <stdlib.h>
#include <string.h>

// One dialog request. Strings are owned by Go; result and shown by GLib.
typedef struct {
	int mode; // 0 open, 1 open several, 2 save
	char *title;
	char *dir;
	char *name;
	char *filter_name;
	char *patterns; // newline-separated
	char *pick;     // tests: answer the dialog with this file
	char **result;
	int nresult;
	char *shown;    // folder when the window mapped
	int done;
	GMutex lock;
	GCond cond;
	GtkWidget *dialog;
} kbv_req;

// The folder and name are set before the window is shown and again after it
// maps, because GTK can repaint the chooser on map.
static void kbv_place(kbv_req *r) {
	GtkFileChooser *fc = GTK_FILE_CHOOSER(r->dialog);
	if (r->dir != NULL && r->dir[0] != 0 && g_file_test(r->dir, G_FILE_TEST_IS_DIR)) {
		gchar *cur = gtk_file_chooser_get_current_folder(fc);
		if (cur == NULL || strcmp(cur, r->dir) != 0) {
			gtk_file_chooser_set_current_folder(fc, r->dir);
		}
		g_free(cur);
	}
	if (r->mode == 2 && r->name != NULL && r->name[0] != 0) {
		gchar *cur = gtk_file_chooser_get_current_name(fc);
		if (cur == NULL || cur[0] == 0) {
			gtk_file_chooser_set_current_name(fc, r->name);
		}
		g_free(cur);
	}
}

static gboolean kbv_place_idle(gpointer data) {
	kbv_place(data);
	return G_SOURCE_REMOVE;
}

static gboolean kbv_accept(gpointer data) {
	kbv_req *r = data;
	if (r->mode != 2) {
		GSList *l = gtk_file_chooser_get_filenames(GTK_FILE_CHOOSER(r->dialog));
		if (l == NULL) {
			return G_SOURCE_CONTINUE;
		}
		g_slist_free_full(l, g_free);
	}
	gtk_dialog_response(GTK_DIALOG(r->dialog), GTK_RESPONSE_ACCEPT);
	return G_SOURCE_REMOVE;
}

static gboolean kbv_answer(gpointer data) {
	kbv_req *r = data;
	if (r->mode != 2) {
		gtk_file_chooser_select_filename(GTK_FILE_CHOOSER(r->dialog), r->pick);
	}
	g_timeout_add(100, kbv_accept, r);
	return G_SOURCE_REMOVE;
}

static void kbv_on_map(GtkWidget *w, gpointer data) {
	kbv_req *r = data;
	if (r->shown == NULL) {
		gchar *f = gtk_file_chooser_get_current_folder(GTK_FILE_CHOOSER(w));
		r->shown = f != NULL ? f : g_strdup("");
	}
	kbv_place(r);
	g_idle_add(kbv_place_idle, r);
	if (r->pick != NULL) {
		g_timeout_add(300, kbv_answer, r);
	}
}

static GtkWindow *kbv_parent(void) {
	GtkWindow *found = NULL;
	GList *all = gtk_window_list_toplevels();
	for (GList *l = all; l != NULL; l = l->next) {
		GtkWindow *w = l->data;
		if (gtk_widget_get_visible(GTK_WIDGET(w)) && !GTK_IS_DIALOG(w) &&
		    gtk_window_get_window_type(w) == GTK_WINDOW_TOPLEVEL) {
			found = w;
			break;
		}
	}
	g_list_free(all);
	return found;
}

static void kbv_drain(void) {
	while (gtk_events_pending()) {
		gtk_main_iteration();
	}
}

static gboolean kbv_run(gpointer data) {
	kbv_req *r = data;
	GtkFileChooserAction action = r->mode == 2 ? GTK_FILE_CHOOSER_ACTION_SAVE : GTK_FILE_CHOOSER_ACTION_OPEN;
	r->dialog = gtk_file_chooser_dialog_new(r->title, kbv_parent(), action,
		"_Cancel", GTK_RESPONSE_CANCEL,
		r->mode == 2 ? "_Save" : "_Open", GTK_RESPONSE_ACCEPT,
		NULL);
	GtkFileChooser *fc = GTK_FILE_CHOOSER(r->dialog);
	gtk_dialog_set_default_response(GTK_DIALOG(r->dialog), GTK_RESPONSE_ACCEPT);
	gtk_file_chooser_set_local_only(fc, TRUE);
	if (r->mode == 1) {
		gtk_file_chooser_set_select_multiple(fc, TRUE);
	}
	if (r->mode == 2) {
		gtk_file_chooser_set_do_overwrite_confirmation(fc, TRUE);
		gtk_file_chooser_set_create_folders(fc, TRUE);
	}
	if (r->filter_name != NULL && r->filter_name[0] != 0 && r->patterns != NULL && r->patterns[0] != 0) {
		GtkFileFilter *chosen = gtk_file_filter_new();
		gtk_file_filter_set_name(chosen, r->filter_name);
		gchar **list = g_strsplit(r->patterns, "\n", -1);
		for (gchar **p = list; *p != NULL; p++) {
			if ((*p)[0] != 0) {
				gtk_file_filter_add_pattern(chosen, *p);
			}
		}
		g_strfreev(list);
		gtk_file_chooser_add_filter(fc, chosen);
		GtkFileFilter *everything = gtk_file_filter_new();
		gtk_file_filter_set_name(everything, "All files");
		gtk_file_filter_add_pattern(everything, "*");
		gtk_file_chooser_add_filter(fc, everything);
	}
	kbv_place(r);
	kbv_drain();
	g_signal_connect(r->dialog, "map", G_CALLBACK(kbv_on_map), r);

	if (gtk_dialog_run(GTK_DIALOG(r->dialog)) == GTK_RESPONSE_ACCEPT) {
		GSList *l = gtk_file_chooser_get_filenames(fc);
		if (l == NULL) {
			gchar *one = gtk_file_chooser_get_filename(fc);
			if (one != NULL) {
				l = g_slist_append(NULL, one);
			}
		}
		if (l == NULL && r->mode == 2) {
			gchar *folder = gtk_file_chooser_get_current_folder(fc);
			gchar *name = gtk_file_chooser_get_current_name(fc);
			if (folder != NULL && name != NULL && name[0] != 0) {
				l = g_slist_append(NULL, g_build_filename(folder, name, NULL));
			}
			g_free(folder);
			g_free(name);
		}
		r->nresult = (int)g_slist_length(l);
		if (r->nresult > 0) {
			r->result = calloc(r->nresult, sizeof(char *));
			int i = 0;
			for (GSList *it = l; it != NULL; it = it->next) {
				r->result[i++] = it->data;
			}
		}
		g_slist_free(l);
	}
	gtk_widget_hide(r->dialog);
	kbv_drain();
	gtk_widget_destroy(r->dialog);

	g_mutex_lock(&r->lock);
	r->done = 1;
	g_cond_signal(&r->cond);
	g_mutex_unlock(&r->lock);
	return G_SOURCE_REMOVE;
}

// kbv_pick runs the dialog on the GTK main loop and waits for it.
static void kbv_pick(kbv_req *r) {
	g_mutex_init(&r->lock);
	g_cond_init(&r->cond);
	g_idle_add(kbv_run, r);
	g_mutex_lock(&r->lock);
	while (!r->done) {
		g_cond_wait(&r->cond, &r->lock);
	}
	g_mutex_unlock(&r->lock);
	g_mutex_clear(&r->lock);
	g_cond_clear(&r->cond);
}

static char *kbv_result(kbv_req *r, int i) { return r->result[i]; }

static void kbv_free(kbv_req *r) {
	for (int i = 0; i < r->nresult; i++) {
		g_free(r->result[i]);
	}
	free(r->result);
	g_free(r->shown);
}

static int kbv_loop(void) {
	if (!gtk_init_check(NULL, NULL)) {
		return 0;
	}
	return 1;
}

static void kbv_main(void) { gtk_main(); }
*/
import "C"

import (
	"os"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

// One dialog at a time; a second call waits for the first to close.
var dialogLock sync.Mutex

// pickFiles opens a GTK file dialog on the window's main loop.
// A cancel is an empty list.
func (d *Dialogs) pickFiles(mode, title, dir, name, filter string, patterns []string) ([]string, error) {
	paths, _ := pickGTK(mode, title, dir, name, filter, patterns)
	return paths, nil
}

// pickGTK also returns the folder the dialog showed when it opened.
// KBV_DIALOG_PICK answers the dialog with that file, for tests.
func pickGTK(mode, title, dir, name, filter string, patterns []string) ([]string, string) {
	dialogLock.Lock()
	defer dialogLock.Unlock()

	r := (*C.kbv_req)(C.calloc(1, C.sizeof_kbv_req))
	defer C.free(unsafe.Pointer(r))
	switch mode {
	case "multi":
		r.mode = 1
	case "save":
		r.mode = 2
	}
	strs := []**C.char{&r.title, &r.dir, &r.name, &r.filter_name, &r.patterns}
	vals := []string{title, dir, name, filter, strings.Join(patterns, "\n")}
	for i, s := range vals {
		*strs[i] = C.CString(s)
		defer C.free(unsafe.Pointer(*strs[i]))
	}
	if pick := os.Getenv("KBV_DIALOG_PICK"); pick != "" {
		r.pick = C.CString(pick)
		defer C.free(unsafe.Pointer(r.pick))
	}

	C.kbv_pick(r)
	defer C.kbv_free(r)

	var paths []string
	for i := 0; i < int(r.nresult); i++ {
		paths = append(paths, C.GoString(C.kbv_result(r, C.int(i))))
	}
	shown := ""
	if r.shown != nil {
		shown = C.GoString(r.shown)
	}
	return paths, shown
}

// startGTK runs a GTK main loop on its own thread, as the window does.
// Tests use it. It reports false when there is no display.
func startGTK() bool {
	ready := make(chan bool)
	go func() {
		runtime.LockOSThread()
		if C.kbv_loop() == 0 {
			ready <- false
			return
		}
		ready <- true
		C.kbv_main()
	}()
	return <-ready
}
