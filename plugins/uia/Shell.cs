// Backing-resource identity for Windows Explorer items.
//
// The gap this closes was found by running the live rename scenario. UI Automation reports
// an Explorer list item as a control captioned "Alpha.txt" and nothing more — no path, no
// parsing name, no shell identity. The Director's binding layer refused it, correctly: a
// caption is not identity, two folders can hold the same name, and "rename this file"
// aimed at a caption is how the wrong file gets renamed.
//
// # Why the shell view, and not something else
//
// The options, in the order the milestone prefers them:
//
//   - A canonical FILESYSTEM PATH is the strongest identity, and Explorer's own automation
//     model hands one over. IShellFolderViewDual — reached through Shell.Application's
//     window list — exposes the view's current folder and its selected items, and each
//     FolderItem carries Path, IsFolder, IsLink and IsFileSystem. That is a path obtained
//     FROM THE SHELL for the item the shell says is selected, not a path assembled from a
//     folder and a caption.
//   - Raw UIA properties do not carry it. Explorer's items expose no ValuePattern and no
//     useful AutomationId; that is what the live run demonstrated.
//   - IShellItem/IShellFolder and PIDLs would work and are far more machinery: COM
//     interface declarations, manual marshalling, and lifetime rules to get wrong. The
//     view's automation model answers the same question with none of it.
//
// So this is the NARROWEST reliable implementation, and everything it produces is either a
// real path the shell reported or nothing at all.
//
// # Everything here fails closed
//
// No current folder, no selection, several selected items, a virtual item with no
// filesystem path, a caption that disagrees with the shell's own name for the item, a path
// that does not canonicalise, an item that vanished mid-observation: every one of them
// yields NO resource and a recorded reason. The Director then refuses, which is the
// outcome that was already correct before this file existed.
//
// # Late binding on purpose
//
// Everything below goes through reflection rather than COM interop types. The build is a
// single csc.exe invocation with no SDK and no NuGet (see build.ps1), and adding a
// Shell32/SHDocVw interop reference would mean either a generated interop assembly to ship
// or a tlbimp step to keep in sync. Reflection over IDispatch needs neither.

using System;
using System.Collections.Generic;
using System.IO;
using System.Reflection;
using System.Runtime.InteropServices;

namespace MarcoUia
{
    // ShellResource is one item's backing identity, as the shell reports it.
    //
    // Serialised into the observation. There are no COM pointers, no PIDLs and no
    // coordinates here: every field is a string a later process can act on, and a handle
    // that meant something only inside this walk would be worse than nothing.
    internal sealed class ShellResource
    {
        // Kind is "file" or "folder". Derived from the shell's own IsFolder, never from
        // the caption — a file called "Reports" and a folder called "Reports.txt" both
        // exist, and only the shell knows which is which.
        public string Kind = "";
        // Path is the canonical filesystem path. Empty means there is no resource, which
        // is the only honest answer for a virtual item.
        public string Path = "";
        // ParsingName is the shell's own round-trippable name for the item. For a
        // filesystem item it equals Path; for anything else it is what the shell would
        // parse back, and it is recorded as evidence rather than acted on.
        public string ParsingName = "";
        // DisplayName is what the shell calls it, which is what the caption should match.
        // Kept so a correlation failure can say WHICH two names disagreed.
        public string DisplayName = "";
        // Source names how this was obtained, so a reader can weigh it.
        public string Source = "";
        // Confidence is 0..1.
        public double Confidence;
        // Link marks a shortcut. Reported, and deliberately NOT followed — see the
        // shortcut rule in the walk below.
        public bool Link;
        // Evidence explains how the identity was established, one clause each.
        public List<string> Evidence = new List<string>();
    }

    // ShellView is what one Explorer window's view reports.
    internal sealed class ShellView
    {
        // FolderPath is the folder the view is currently showing, empty when it is not a
        // filesystem folder.
        public string FolderPath = "";
        public string FolderParsingName = "";
        // WindowId identifies the window this came from.
        public string WindowId = "";
        // SelectedCount is how many items the SHELL says are selected. More than one is an
        // ambiguity, not a choice.
        public int SelectedCount;
        // Selected is the single selected item, null unless SelectedCount is exactly 1 and
        // it has a trustworthy filesystem path.
        public ShellResource Selected;
        // Refusal explains why there is no Selected, empty when there is one.
        public string Refusal = "";
        // Available reports whether a shell view was found for this window at all.
        public bool Available;
    }

    internal static class ShellItems
    {
        // ForWindow returns what the shell says about one Explorer window's view.
        //
        // Never throws: a shell that is busy, a window that closed between enumeration and
        // interrogation, or a COM call that fails is reported as an unavailable view. The
        // walk carries on and the Director sees an item with no resource, which it already
        // knows how to refuse.
        public static ShellView ForWindow(IntPtr hwnd, string app)
        {
            var view = new ShellView { WindowId = "hwnd:" + hwnd.ToInt64().ToString() };

            // Only Explorer. The shell window list also contains Internet Explorer
            // windows, and asking one of those for a folder is meaningless.
            if (app != "explorer")
            {
                view.Refusal = "not a File Explorer window";
                return view;
            }

            object shell = null;
            try
            {
                var t = Type.GetTypeFromProgID("Shell.Application");
                if (t == null)
                {
                    view.Refusal = "the shell automation object is not registered";
                    return view;
                }
                shell = Activator.CreateInstance(t);
                object windows = Invoke(shell, "Windows");
                if (windows == null)
                {
                    view.Refusal = "the shell exposes no window list";
                    return view;
                }

                long want = hwnd.ToInt64();
                foreach (object w in (System.Collections.IEnumerable)windows)
                {
                    try
                    {
                        object h = Get(w, "HWND");
                        if (h == null || Convert.ToInt64(h) != want) continue;
                        Populate(view, w);
                        return view;
                    }
                    catch (Exception)
                    {
                        // A window that went away mid-enumeration. Skip it; the loop is
                        // looking for one particular handle and this was not it.
                    }
                    finally { Release(w); }
                }
                view.Refusal = "no shell view is registered for this window";
                return view;
            }
            catch (Exception e)
            {
                view.Refusal = "the shell could not be asked: " + e.Message;
                return view;
            }
            finally { Release(shell); }
        }

        // Populate reads one shell window's folder and selection.
        static void Populate(ShellView view, object window)
        {
            object doc = null, folder = null, self = null, selection = null;
            try
            {
                doc = Get(window, "Document");
                if (doc == null)
                {
                    view.Refusal = "the window exposes no view document";
                    return;
                }
                folder = Get(doc, "Folder");
                if (folder == null)
                {
                    view.Refusal = "the view is showing no folder";
                    return;
                }
                view.Available = true;

                // The CURRENT FOLDER, from the view itself. Everything else is judged
                // against it: an item whose path is not inside this folder did not come
                // from this view, whatever it is called.
                self = Get(folder, "Self");
                if (self != null)
                {
                    view.FolderPath = Canonical(AsString(Get(self, "Path")));
                    view.FolderParsingName = AsString(Get(self, "Path"));
                    if (!IsTrue(Get(self, "IsFileSystem")))
                    {
                        // A virtual location — This PC, a library, a search result. It has
                        // no folder path, and an item "in" it cannot be given one.
                        view.FolderPath = "";
                        view.Refusal = "the view is showing a virtual location, which has " +
                            "no folder on disk";
                        return;
                    }
                }
                if (view.FolderPath == "")
                {
                    view.Refusal = "the view's current folder could not be established";
                    return;
                }

                selection = Invoke(doc, "SelectedItems");
                if (selection == null)
                {
                    view.Refusal = "the view reports no selection";
                    return;
                }
                object count = Get(selection, "Count");
                view.SelectedCount = count == null ? 0 : Convert.ToInt32(count);

                if (view.SelectedCount == 0)
                {
                    view.Refusal = "nothing is selected in this view";
                    return;
                }
                if (view.SelectedCount > 1)
                {
                    // AMBIGUOUS, not a choice. Picking the first would act on whichever
                    // the shell happened to list first.
                    view.Refusal = view.SelectedCount + " items are selected, so none of " +
                        "them is the one that was meant";
                    return;
                }

                object item = Invoke(selection, "Item", 0);
                if (item == null)
                {
                    view.Refusal = "the selected item could not be read";
                    return;
                }
                try { view.Selected = Describe(item, view); }
                finally { Release(item); }
            }
            catch (Exception e)
            {
                view.Refusal = "the view could not be read: " + e.Message;
            }
            finally
            {
                Release(selection);
                Release(self);
                Release(folder);
                Release(doc);
            }
        }

        // Describe turns one FolderItem into a resource, or into nothing with a reason.
        static ShellResource Describe(object item, ShellView view)
        {
            string name = AsString(Get(item, "Name"));
            bool fileSystem = IsTrue(Get(item, "IsFileSystem"));
            bool isFolder = IsTrue(Get(item, "IsFolder"));
            bool isLink = IsTrue(Get(item, "IsLink"));
            string raw = AsString(Get(item, "Path"));

            if (!fileSystem)
            {
                // A virtual item — a control panel entry, a library, a search result.
                // It has no path, and inventing one from the folder and the caption is
                // exactly the synthesis this whole file exists to avoid.
                view.Refusal = "the selected item \"" + name + "\" is a shell object with " +
                    "no file behind it";
                return null;
            }

            string path = Canonical(raw);
            if (path == "")
            {
                view.Refusal = "the selected item \"" + name + "\" reports no usable path";
                return null;
            }

            // The path must be INSIDE the folder the view is showing. A disagreement means
            // the selection and the view came from different places, and neither can be
            // trusted to describe the other.
            if (!Within(view.FolderPath, path))
            {
                view.Refusal = "the selected item's path (" + path + ") is not inside the " +
                    "folder this view is showing (" + view.FolderPath + ")";
                return null;
            }

            // And it must still be there. An item that vanished between the shell listing
            // it and this line is not something to act on.
            bool existsAsFile = File.Exists(path);
            bool existsAsDir = Directory.Exists(path);
            if (!existsAsFile && !existsAsDir)
            {
                view.Refusal = "the selected item \"" + name + "\" is no longer there";
                return null;
            }

            var res = new ShellResource
            {
                Path = path,
                ParsingName = raw,
                DisplayName = name,
                Link = isLink,
                Source = "shell_folder_view",
                Confidence = 1.0,
            };

            // Kind comes from the FILESYSTEM, corroborated by the shell. They disagree
            // only when something changed underneath, and disagreement is a refusal.
            if (isFolder != existsAsDir)
            {
                view.Refusal = "the shell and the filesystem disagree about whether \"" +
                    name + "\" is a folder";
                return null;
            }
            res.Kind = existsAsDir ? "folder" : "file";

            res.Evidence.Add("the shell reports this item as selected in " + view.FolderPath);
            res.Evidence.Add("its path is " + path);
            res.Evidence.Add(res.Kind == "folder"
                ? "it is a directory on disk"
                : "it is a file on disk");
            if (isLink)
            {
                // A shortcut binds to the SHORTCUT FILE, never to what it points at.
                // Following it would mean "rename this file" renamed the target, which is
                // a different file in a different folder that the user is not looking at.
                res.Evidence.Add("it is a shortcut; the identity is the shortcut file " +
                    "itself, not what it points at");
            }
            return res;
        }

        // Within reports whether a path sits directly inside a folder.
        //
        // DIRECTLY: a file in a subfolder is not an item of this view, and treating it as
        // one would let a selection in a different pane look like this one.
        static bool Within(string folder, string path)
        {
            if (folder == "" || path == "") return false;
            string parent;
            try { parent = Canonical(System.IO.Path.GetDirectoryName(path)); }
            catch (Exception) { return false; }
            return string.Equals(parent, folder, StringComparison.OrdinalIgnoreCase);
        }

        // Canonical resolves a path to its full form, "" when it cannot be.
        //
        // GetFullPath rather than the raw string: the shell may hand back a path with a
        // trailing separator or a relative segment, and two spellings of one file must
        // compare equal or the binding layer will think the object moved.
        public static string Canonical(string path)
        {
            if (string.IsNullOrEmpty(path)) return "";
            try
            {
                string full = System.IO.Path.GetFullPath(path);
                if (full.Length > 3) full = full.TrimEnd('\\', '/');
                return full;
            }
            catch (Exception) { return ""; }
        }

        // ── late-bound COM helpers ────────────────────────────────────────────

        static object Get(object target, string property)
        {
            if (target == null) return null;
            return target.GetType().InvokeMember(property, BindingFlags.GetProperty,
                null, target, null);
        }

        static object Invoke(object target, string method, params object[] args)
        {
            if (target == null) return null;
            return target.GetType().InvokeMember(method, BindingFlags.InvokeMethod,
                null, target, args);
        }

        static string AsString(object v) { return v == null ? "" : v.ToString(); }

        static bool IsTrue(object v)
        {
            if (v == null) return false;
            try { return Convert.ToBoolean(v); }
            catch (Exception) { return false; }
        }

        // Release drops a COM reference. Explorer's view objects are held by the shell,
        // and leaking them across a walk that runs after every action would keep windows
        // alive that the user has closed.
        static void Release(object o)
        {
            if (o != null && Marshal.IsComObject(o))
            {
                try { Marshal.ReleaseComObject(o); } catch (Exception) { }
            }
        }
    }
}
