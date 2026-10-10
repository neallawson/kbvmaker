# GTK file dialog. The folder and name are set before the window is shown and
# again after it maps, because GTK can repaint the chooser on map.
#
# KBV_DIALOG_PICK makes the dialog answer itself for tests: it writes the folder
# shown when the window maps to stderr, selects that file (save keeps the
# default name), and accepts.
import os
import sys

import gi

gi.require_version("Gtk", "3.0")
from gi.repository import GLib, Gtk


def main():
    mode, title, directory, filename, filter_name = sys.argv[1:6]
    patterns = [p for p in sys.argv[6:] if p]
    action = Gtk.FileChooserAction.SAVE if mode == "save" else Gtk.FileChooserAction.OPEN
    dialog = Gtk.FileChooserDialog(title=title, action=action)
    dialog.add_button("_Cancel", Gtk.ResponseType.CANCEL)
    dialog.add_button("_Save" if mode == "save" else "_Open", Gtk.ResponseType.ACCEPT)
    dialog.set_default_response(Gtk.ResponseType.ACCEPT)
    dialog.set_local_only(True)
    if mode == "multi":
        dialog.set_select_multiple(True)
    if mode == "save":
        dialog.set_do_overwrite_confirmation(True)
        dialog.set_create_folders(True)
    if filter_name and patterns:
        chosen = Gtk.FileFilter()
        chosen.set_name(filter_name)
        for pattern in patterns:
            chosen.add_pattern(pattern)
        dialog.add_filter(chosen)
        everything = Gtk.FileFilter()
        everything.set_name("All files")
        everything.add_pattern("*")
        dialog.add_filter(everything)

    def place():
        if directory and os.path.isdir(directory):
            if dialog.get_current_folder() != directory:
                dialog.set_current_folder(directory)
        if mode == "save" and filename and not dialog.get_current_name():
            dialog.set_current_name(filename)
        return False

    place()
    while Gtk.events_pending():
        Gtk.main_iteration_do(False)

    pick = os.environ.get("KBV_DIALOG_PICK")
    first = {}

    def answer():
        sys.stderr.write("folder: %s\n" % first.get("folder"))
        sys.stderr.flush()
        if mode != "save":
            dialog.select_filename(pick)

        def accept():
            if mode != "save" and not dialog.get_filenames():
                return True
            dialog.response(Gtk.ResponseType.ACCEPT)
            return False

        GLib.timeout_add(100, accept)
        return False

    def on_map(_widget):
        first.setdefault("folder", dialog.get_current_folder())
        place()
        GLib.idle_add(place)
        if pick:
            GLib.timeout_add(300, answer)
        return False

    dialog.connect("map", on_map)
    response = dialog.run()
    if response != Gtk.ResponseType.ACCEPT:
        dialog.destroy()
        return 1
    names = dialog.get_filenames() or []
    if not names and dialog.get_filename():
        names = [dialog.get_filename()]
    if mode == "save" and not names:
        folder = dialog.get_current_folder()
        name = dialog.get_current_name()
        if folder and name:
            names = [os.path.join(folder, name)]
    dialog.destroy()
    if not names:
        return 1
    for name in names:
        print(name)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
