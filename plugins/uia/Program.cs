// Command uia is the Marco accessibility resolver: a bridge host (see spec/Hosts.md)
// that fulfils an `Accessibility` act by reading the operating system's UI Automation
// tree. It is the Director's strongest routinely available perception source — the
// one that reports an element IS a button, IS labelled "Save" and IS disabled,
// rather than that some pixels look that way.
//
// It speaks the bridge JSON protocol on stdio, one object per line:
//
//   → {"act":"Accessibility","action":"Snapshot","input":{"Window":"hwnd:1234"}}
//   ← {"status":"ok","data":{"WindowId":"hwnd:1234","App":"notepad","Elements":[…]}}
//   → {"act":"Accessibility","action":"Available"}
//   ← {"status":"ok","data":{"Available":true}}
//
// Snapshot's input is all optional: Window (default: the foreground window),
// MaxNodes, MaxDepth, TimeoutMs. Omitting everything gives a bounded snapshot of
// whatever is in front, which is the normal case and the only one fast enough to run
// on every observation.
//
// It is written in C# because managed UI Automation makes the tree walk
// straightforward, where hand-rolled COM vtable calls from Go would not be — and it
// is a SUBPROCESS so that choice never reaches Marco's zero-dependency engine. The
// Director sees it only through directorapi.AccessibilityProvider, so replacing it
// with an in-process implementation later is an optimisation, not a rewrite.
//
// Build (no SDK, no NuGet — the csc.exe every Windows machine already has):
//
//   powershell -File plugins/uia/build.ps1
//
// Wire it into a Director run with --accessibility bridge:plugins/uia/uia.exe.

using System;
using System.Collections.Generic;
using System.IO;
using System.Text;

namespace MarcoUia
{
    internal static partial class Program
    {
        static int Main(string[] args)
        {
            // Before anything reads a coordinate: on a scaled display an unaware
            // process gets virtualised coordinates, and every bound we reported would
            // be wrong by the scale factor.
            Native.MakeDpiAware();

            // `uia snapshot [out.json]` is a one-shot debug/fixture dump; any other
            // invocation is the bridge host reading JSON on stdin.
            if (args.Length > 0 && args[0] == "snapshot")
                return RunSnapshotDump(args);

            var stdout = new StreamWriter(Console.OpenStandardOutput(), new UTF8Encoding(false));
            stdout.AutoFlush = false;

            string line;
            while ((line = Console.In.ReadLine()) != null)
            {
                if (line.Trim().Length == 0) continue;
                string reply;
                try { reply = Handle(line); }
                catch (Exception e) { reply = Failed(e.Message); }
                stdout.Write(reply);
                stdout.Write('\n');
                stdout.Flush(); // the caller is waiting on this line
            }
            return 0;
        }

        static string Handle(string line)
        {
            var req = JsonReader.Parse(line) as Dictionary<string, object>;
            if (req == null) return Failed("malformed request");

            string action = JsonReader.Str(req, "action");
            var input = JsonReader.Obj(req, "input");

            switch (action.ToLowerInvariant())
            {
                case "snapshot":
                    return SnapshotReply(input);
                case "focus":
                    return FocusReply(input);
                case "setvalue":
                    return ValueReply(input, true);
                case "getvalue":
                    return ValueReply(input, false);
                // The structural actions. Each names a semantic operation the
                // application performs itself; the Director has already decided this is
                // the right mechanism, and a control that cannot do it answers
                // "unsupported:" so the Director can fall back and RECORD that it did.
                case "invoke":
                    return StructuralReply(input, "invoke");
                case "expand":
                    return StructuralReply(input, "expand");
                case "collapse":
                    return StructuralReply(input, "collapse");
                case "toggle":
                    return StructuralReply(input, "toggle");
                case "select":
                    return StructuralReply(input, "select");
                case "deselect":
                    return StructuralReply(input, "deselect");
                case "scrollintoview":
                    return StructuralReply(input, "scrollintoview");
                case "available":
                    // Reported by attempting the cheapest real operation rather than
                    // by assuming: the Director needs to know it can degrade
                    // deliberately, not discover it by timeout mid-plan.
                    return AvailableReply();
                default:
                    return Failed("Accessibility has no action \"" + action + "\"");
            }
        }

        /// <summary>Builds walk options from the request. Throws on a Window that was
        /// given but cannot be understood.
        ///
        /// That throw matters. Falling back to the foreground window would answer a
        /// question about window X with a description of window Y — and the caller
        /// has no way to tell, because the reply looks perfectly successful. The
        /// Director would then plan against the wrong application entirely. An
        /// unusable scope is an error, not a default.</summary>
        static WalkOptions OptionsFrom(Dictionary<string, object> input)
        {
            var opts = new WalkOptions();
            if (input == null) return opts;

            opts.MaxNodes = JsonReader.Int(input, "MaxNodes", opts.MaxNodes);
            opts.MaxDepth = JsonReader.Int(input, "MaxDepth", opts.MaxDepth);
            opts.TimeoutMs = JsonReader.Int(input, "TimeoutMs", opts.TimeoutMs);

            string w = JsonReader.Str(input, "Window");
            if (w.Length == 0) return opts; // unscoped: the foreground window, as documented

            if (!w.StartsWith("hwnd:", StringComparison.OrdinalIgnoreCase))
                throw new ArgumentException("Window must look like \"hwnd:<handle>\", got \"" + w + "\"");

            long h;
            if (!long.TryParse(w.Substring(5), out h) || h == 0)
                throw new ArgumentException("Window \"" + w + "\" is not a usable handle");

            opts.Window = new IntPtr(h);
            return opts;
        }

        static string SnapshotReply(Dictionary<string, object> input)
        {
            Snapshot snap = Walker.Snapshot(OptionsFrom(input));

            var sb = new StringBuilder(64 * 1024);
            var w = new JsonWriter(sb);
            w.ObjStart();
            w.Field("status", "ok"); w.Sep();
            w.Key("data");
            WriteSnapshot(w, snap);
            w.ObjEnd();
            return sb.ToString();
        }

        static void WriteSnapshot(JsonWriter w, Snapshot snap)
        {
            w.ObjStart();
            w.Field("WindowId", snap.WindowId); w.Sep();
            w.Field("WindowTitle", snap.WindowTitle); w.Sep();
            w.Field("App", snap.App); w.Sep();
            w.Field("ProcessId", snap.ProcessId); w.Sep();
            w.Field("WindowX", snap.WX); w.Sep();
            w.Field("WindowY", snap.WY); w.Sep();
            w.Field("WindowW", snap.WW); w.Sep();
            w.Field("WindowH", snap.WH); w.Sep();
            w.Field("Minimized", snap.Minimized); w.Sep();
            w.Field("Maximized", snap.Maximized); w.Sep();
            w.Field("WindowVisible", snap.WindowVisible); w.Sep();
            // Partial and Reason are load-bearing: they are how a bounded walk says
            // "I stopped looking" instead of "there is nothing there".
            w.Field("Partial", snap.Partial); w.Sep();
            w.Field("Reason", snap.Reason); w.Sep();
            w.Field("ElapsedMs", (int)snap.ElapsedMs); w.Sep();

            // What the window's own shell view reported. Diagnostics: which folder was
            // being shown, how many items the shell considered selected, and — when no
            // resource reached any node — exactly why not. Omitted for a window that has
            // no shell view, which is every window that is not File Explorer.
            if (snap.Shell != null)
            {
                w.Key("Shell");
                w.ObjStart();
                w.Field("Available", snap.Shell.Available); w.Sep();
                w.Field("FolderPath", snap.Shell.FolderPath); w.Sep();
                w.Field("SelectedCount", snap.Shell.SelectedCount); w.Sep();
                w.Field("Refusal", snap.Shell.Refusal);
                w.ObjEnd();
                w.Sep();
            }

            w.Key("Elements");
            w.ArrStart();
            for (int i = 0; i < snap.Nodes.Count; i++)
            {
                if (i > 0) w.Sep();
                WriteNode(w, snap.Nodes[i]);
            }
            w.ArrEnd();
            w.ObjEnd();
        }

        static void WriteNode(JsonWriter w, Node n)
        {
            w.ObjStart();
            w.Field("Id", n.Id); w.Sep();
            w.Field("ParentId", n.ParentId ?? ""); w.Sep();
            w.Field("Role", n.Role); w.Sep();
            w.Field("ControlType", n.ControlType); w.Sep();
            w.Field("Label", n.Label); w.Sep();
            w.Field("Value", n.Value); w.Sep();
            w.Field("Description", n.Description); w.Sep();
            w.Field("X", n.X); w.Sep();
            w.Field("Y", n.Y); w.Sep();
            w.Field("W", n.W); w.Sep();
            w.Field("H", n.H); w.Sep();
            w.Field("Enabled", n.Enabled); w.Sep();
            w.Field("Visible", n.Visible); w.Sep();
            w.Field("Focused", n.Focused); w.Sep();
            w.Field("Selected", n.Selected); w.Sep();
            w.Field("Offscreen", n.Offscreen); w.Sep();
            w.Field("AutomationId", n.AutomationId); w.Sep();
            w.Field("ClassName", n.ClassName); w.Sep();
            // The capability evidence the Director's ladder reads. Sent as a
            // comma-separated string rather than an array so the existing reader — which
            // decodes flat fields — needs no new shape, and an empty string means "this
            // control implements none of the ones we look for" rather than "unknown":
            // the field is always present, so its absence would be the bridge being old,
            // not the control being featureless.
            w.Field("Patterns", string.Join(",", n.Patterns.ToArray())); w.Sep();
            w.Field("Expanded", n.Expanded); w.Sep();
            w.Field("Checked", n.Checked); w.Sep();
            w.Field("Depth", n.Depth);
            // The object behind the control, when one was established. OMITTED entirely
            // when there is none, which is almost every control: a reader must be able to
            // tell "this has no file behind it" from "this bridge is old", and an always-
            // present empty object would collapse the two.
            if (n.Resource != null)
            {
                w.Sep();
                w.Key("Resource");
                WriteResource(w, n.Resource);
            }
            w.ObjEnd();
        }

        static void WriteResource(JsonWriter w, ShellResource r)
        {
            w.ObjStart();
            w.Field("Kind", r.Kind); w.Sep();
            w.Field("Path", r.Path); w.Sep();
            w.Field("ParsingName", r.ParsingName); w.Sep();
            w.Field("DisplayName", r.DisplayName); w.Sep();
            w.Field("Source", r.Source); w.Sep();
            w.Field("Confidence", r.Confidence); w.Sep();
            w.Field("Link", r.Link); w.Sep();
            // Comma-separated for the same reason Patterns is: the reader decodes flat
            // fields, and one shape is easier to keep honest than two.
            w.Field("Evidence", string.Join(" | ", r.Evidence.ToArray()));
            w.ObjEnd();
        }

        /// <summary>Focus moves keyboard focus to an element the Director has already
        /// chosen, named by the same id its snapshot reported.
        ///
        /// This is the one ACTION this plugin performs. It is here rather than in
        /// Marco's mouse/keyboard host because focusing is not a click: clicking a
        /// control activates it, and "focus the search box" must not press anything.
        /// There is no way to express that with input injection.</summary>
        static string FocusReply(Dictionary<string, object> input)
        {
            string id = JsonReader.Str(input, "Element");
            if (id.Length == 0) return Failed("focus needs an Element id");

            IntPtr hwnd = IntPtr.Zero;
            string w = JsonReader.Str(input, "Window");
            if (w.Length > 0)
            {
                long h;
                if (!w.StartsWith("hwnd:", StringComparison.OrdinalIgnoreCase) ||
                    !long.TryParse(w.Substring(5), out h) || h == 0)
                    return Failed("Window must look like \"hwnd:<handle>\", got \"" + w + "\"");
                hwnd = new IntPtr(h);
            }

            string problem = Focuser.Focus(hwnd, id, JsonReader.Int(input, "MaxNodes", 1500));
            if (problem.Length > 0) return Failed(problem);

            var sb = new StringBuilder();
            var jw = new JsonWriter(sb);
            jw.ObjStart();
            jw.Field("status", "ok"); jw.Sep();
            jw.Key("data");
            jw.ObjStart();
            jw.Field("Focused", true);
            jw.ObjEnd();
            jw.ObjEnd();
            return sb.ToString();
        }

        /// <summary>Performs one structural action.
        ///
        /// One handler for all seven because they differ only in which pattern they
        /// reach: the element lookup, the window parsing, the unsupported/refused
        /// distinction and the reply shape are identical, and seven copies of them
        /// would be seven places for the "unsupported:" contract to drift.</summary>
        static string StructuralReply(Dictionary<string, object> input, string what)
        {
            string id = JsonReader.Str(input, "Element");
            string called = JsonReader.Str(input, "Name");
            if (id.Length == 0 && called.Length == 0)
                return Failed(what + " needs an Element id or a control's name");
            if (id.Length > 0 && called.Length > 0)
                return Failed(what + " was given both an Element id and a name; a control is " +
                    "identified one way or the other");

            IntPtr hwnd = IntPtr.Zero;
            string w = JsonReader.Str(input, "Window");
            if (w.Length > 0)
            {
                long h;
                if (!w.StartsWith("hwnd:", StringComparison.OrdinalIgnoreCase) ||
                    !long.TryParse(w.Substring(5), out h) || h == 0)
                    return Failed("Window must look like \"hwnd:<handle>\", got \"" + w + "\"");
                hwnd = new IntPtr(h);
            }

            int maxNodes = JsonReader.Int(input, "MaxNodes", 1500);
            // A NAME becomes an id here, against the tree as it stands right now. This is the
            // whole reason a saved play can press a control: it holds the word the person saw,
            // never a runtime id, which would have gone stale by the first repaint. Resolution
            // refuses rather than guesses when nothing matches or several do — see Uia.Resolve.
            if (called.Length > 0)
            {
                string trouble = Focuser.Resolve(hwnd, called, maxNodes, out id);
                if (trouble.Length > 0) return Failed(trouble);
            }
            string problem, state = "";
            switch (what)
            {
                case "invoke":
                    problem = Focuser.Invoke(hwnd, id, maxNodes); break;
                case "expand":
                    problem = Focuser.ExpandCollapse(hwnd, id, true, maxNodes, out state); break;
                case "collapse":
                    problem = Focuser.ExpandCollapse(hwnd, id, false, maxNodes, out state); break;
                case "toggle":
                    problem = Focuser.Toggle(hwnd, id, maxNodes, out state); break;
                case "select":
                    problem = Focuser.Select(hwnd, id, true, maxNodes); break;
                case "deselect":
                    problem = Focuser.Select(hwnd, id, false, maxNodes); break;
                case "scrollintoview":
                    problem = Focuser.ScrollIntoView(hwnd, id, maxNodes); break;
                default:
                    return Failed("Accessibility has no action \"" + what + "\"");
            }
            if (problem.Length > 0) return Failed(problem);

            var sb = new StringBuilder();
            var jw = new JsonWriter(sb);
            jw.ObjStart();
            jw.Field("status", "ok"); jw.Sep();
            jw.Key("data");
            jw.ObjStart();
            jw.Field("Performed", true);
            // The resulting state, for the actions that have one. It is the PROOF a
            // toggle landed where it was meant to — the Director verifies against the
            // world too, but this is the application's own answer.
            if (state.Length > 0) { jw.Sep(); jw.Field("State", state); }
            jw.ObjEnd();
            jw.ObjEnd();
            return sb.ToString();
        }

        static string AvailableReply()
        {
            bool ok;
            string reason = "";
            try
            {
                IntPtr hwnd = Native.GetForegroundWindow();
                ok = hwnd != IntPtr.Zero &&
                     System.Windows.Automation.AutomationElement.FromHandle(hwnd) != null;
                if (!ok) reason = "no foreground automation element";
            }
            catch (Exception e)
            {
                ok = false;
                reason = e.Message;
            }

            var sb = new StringBuilder();
            var w = new JsonWriter(sb);
            w.ObjStart();
            w.Field("status", "ok"); w.Sep();
            w.Key("data");
            w.ObjStart();
            w.Field("Available", ok); w.Sep();
            w.Field("Reason", reason);
            w.ObjEnd();
            w.ObjEnd();
            return sb.ToString();
        }

        static string Failed(string message)
        {
            var sb = new StringBuilder();
            var w = new JsonWriter(sb);
            w.ObjStart();
            w.Field("status", "failed"); w.Sep();
            w.Field("error", message);
            w.ObjEnd();
            return sb.ToString();
        }

        /// <summary>`uia snapshot [out.json] [--delay ms]` dumps one snapshot. This is
        /// how perception fixtures are recorded: run it with a delay, bring the dialog
        /// you want to capture to the front, and the resulting JSON becomes a test
        /// input that the whole downstream pipeline can be developed against with no
        /// live desktop at all.</summary>
        static int RunSnapshotDump(string[] args)
        {
            string outPath = null;
            int delayMs = 0;
            // quiet suppresses the stderr progress line. Scripts that call this in a
            // loop want it: PowerShell turns a native process's stderr into error
            // records, so a chatty tool is a broken pipeline.
            bool quiet = false;
            var opts = new WalkOptions();
            for (int i = 1; i < args.Length; i++)
            {
                if (args[i] == "--quiet")
                {
                    quiet = true;
                }
                else if (args[i] == "--delay" && i + 1 < args.Length)
                {
                    int.TryParse(args[i + 1], out delayMs);
                    i++;
                }
                else if (args[i] == "--window" && i + 1 < args.Length)
                {
                    // Capturing by handle instead of by focus lets a fixture be
                    // recorded without the target window having to be in front —
                    // so recording one doesn't disturb whatever you were doing.
                    long h;
                    if (!long.TryParse(args[i + 1], out h) || h == 0)
                    {
                        Console.Error.WriteLine("uia: --window needs a non-zero window handle");
                        return 2;
                    }
                    opts.Window = new IntPtr(h);
                    i++;
                }
                else if (args[i] == "--max-nodes" && i + 1 < args.Length)
                {
                    opts.MaxNodes = int.Parse(args[i + 1]);
                    i++;
                }
                else if (!args[i].StartsWith("--"))
                {
                    outPath = args[i];
                }
            }

            if (delayMs > 0)
            {
                Console.Error.WriteLine("uia: capturing the foreground window in " +
                                        (delayMs / 1000.0).ToString("0.#") +
                                        "s — bring the window you want to the front now");
                System.Threading.Thread.Sleep(delayMs);
            }

            Snapshot snap = Walker.Snapshot(opts);
            var sb = new StringBuilder(64 * 1024);
            WriteSnapshot(new JsonWriter(sb), snap);
            string json = sb.ToString();

            if (outPath != null)
            {
                File.WriteAllText(outPath, json, new UTF8Encoding(false));
                if (!quiet)
                {
                    Console.Error.WriteLine("uia: wrote " + snap.Nodes.Count + " elements from \"" +
                                            snap.WindowTitle + "\" (" + snap.App + ") to " + outPath +
                                            " in " + snap.ElapsedMs + "ms" +
                                            (snap.Partial ? " [PARTIAL: " + snap.Reason + "]" : ""));
                }
            }
            else
            {
                Console.Out.WriteLine(json);
            }
            return 0;
        }
    }
}
