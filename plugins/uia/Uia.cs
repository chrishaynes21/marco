// The UI Automation tree walk: the Director's strongest routinely available source.
//
// Two things dominate the design here.
//
// PERFORMANCE. Managed UI Automation is notoriously slow when you read properties
// one at a time, because every read is a cross-process call into the target
// application. A dialog with 40 controls and 15 properties each is 600 round trips.
// The fix is a CacheRequest: ask for every property of every node in the subtree in
// ONE bulk fetch, then read from the cache. That is the difference between a
// snapshot taking tens of milliseconds and taking seconds, and the Director
// re-observes after every meaningful step, so it matters a great deal.
//
// ROBUSTNESS. A UI tree is a live thing being walked from another process: elements
// vanish mid-walk, applications stop responding, and a browser can expose tens of
// thousands of nodes. Every property read is guarded, the walk is bounded by node
// count and depth, and hitting a bound sets Partial on the snapshot rather than
// silently truncating. That distinction matters more than it looks: a truncated
// snapshot that claims to be complete would let the Director conclude "the Save
// button does not exist" when the truth is "I stopped looking".

using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;
using System.Windows.Automation;

namespace MarcoUia
{
    internal static class Native
    {
        [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
        [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
        [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr hWnd);
        [DllImport("user32.dll")] public static extern bool IsZoomed(IntPtr hWnd);
        [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
        [DllImport("user32.dll", CharSet = CharSet.Unicode)]
        public static extern int GetWindowTextW(IntPtr hWnd, StringBuilder text, int count);
        [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);

        [StructLayout(LayoutKind.Sequential)]
        public struct RECT { public int Left, Top, Right, Bottom; }

        // DPI awareness must be set before anything reads a coordinate. Without it
        // Windows virtualises coordinates for this process on a scaled display, and
        // every bound we report would be wrong by the scale factor — the Director
        // would click confidently in the wrong place. PER_MONITOR_AWARE_V2 matches
        // what Marco's engine process uses, so the two agree on what a pixel is.
        [DllImport("user32.dll")]
        static extern bool SetProcessDpiAwarenessContext(IntPtr value);
        [DllImport("user32.dll")]
        static extern bool SetProcessDPIAware();

        static readonly IntPtr PerMonitorAwareV2 = new IntPtr(-4);

        public static void MakeDpiAware()
        {
            try { if (SetProcessDpiAwarenessContext(PerMonitorAwareV2)) return; }
            catch (EntryPointNotFoundException) { /* pre-1703 Windows */ }
            try { SetProcessDPIAware(); } catch { }
        }

        public static string WindowTitle(IntPtr hwnd)
        {
            var sb = new StringBuilder(512);
            int n = GetWindowTextW(hwnd, sb, sb.Capacity);
            return n > 0 ? sb.ToString() : "";
        }

        public static string ExeName(IntPtr hwnd)
        {
            try
            {
                uint pid;
                GetWindowThreadProcessId(hwnd, out pid);
                if (pid == 0) return "";
                using (var p = Process.GetProcessById((int)pid))
                {
                    // Marco's app key is the lowercase basename with no extension
                    // (winctx.appName), so routes, skills and the Director all agree
                    // on what "discord" means.
                    return p.ProcessName.ToLowerInvariant();
                }
            }
            catch { return ""; }
        }

        public static int ProcessId(IntPtr hwnd)
        {
            uint pid;
            GetWindowThreadProcessId(hwnd, out pid);
            return (int)pid;
        }
    }

    /// <summary>Options bounding one snapshot.</summary>
    internal sealed class WalkOptions
    {
        public int MaxNodes = 1500;
        public int MaxDepth = 40;
        public int TimeoutMs = 4000;
        /// <summary>Window handle to scope to; IntPtr.Zero means the foreground window.</summary>
        public IntPtr Window = IntPtr.Zero;
    }

    /// <summary>One element, flattened for the wire.</summary>
    internal sealed class Node
    {
        public string Id;
        public string ParentId;
        public string Role;
        public string ControlType;
        public string Label;
        public string Value;
        public string Description;
        public int X, Y, W, H;
        public bool Enabled = true;
        public bool Visible = true;
        public bool Focused;
        public bool Selected;
        public bool Offscreen;
        public string AutomationId;
        public string ClassName;
        public int Depth;

        /// <summary>The patterns this control implements, lower-cased and comma-free.
        ///
        /// This is what turns the Director's capability ladder from a guess into a
        /// decision. Without it every verb has to be attempted optimistically and the
        /// fallback discovered from a failure; with it the Director knows before acting
        /// that this tree node cannot expand, and can say so rather than clicking where
        /// a disclosure arrow would be.
        ///
        /// Reported as a LIST rather than as booleans so a pattern the Director does not
        /// model yet still arrives intact.</summary>
        public List<string> Patterns = new List<string>();

        /// <summary>Expanded and Checked as three-valued strings: "true", "false", or
        /// "" for a control that has no such state.
        ///
        /// Empty is not false. A tree node with no children is neither expanded nor
        /// collapsed, and a Director that read the absence as "collapsed" would report
        /// success for an expand that did nothing.</summary>
        public string Expanded = "";
        public string Checked = "";

        // Resource is the object BEHIND this control, when one could be established and
        // trusted. Null for almost everything — a button has no file behind it, and a
        // control that pretended otherwise would hand the Director a fiction to act on.
        //
        // Today it is set only for the single selected item of a File Explorer view, from
        // the shell's own account of that view. See Shell.cs.
        public ShellResource Resource;
    }

    internal sealed class Snapshot
    {
        public List<Node> Nodes = new List<Node>();
        public bool Partial;
        public string Reason = "";
        public string WindowId = "";
        public string WindowTitle = "";
        public string App = "";
        public int ProcessId;
        public int WX, WY, WW, WH;
        public bool Minimized, Maximized, WindowVisible;

        // Shell is what the window's own view reported, when it has one. Diagnostics: it
        // says which folder was being shown, how many items the shell considered selected,
        // and — when no resource was attached to any node — exactly why not.
        public ShellView Shell;
        public long ElapsedMs;
    }

    /// <summary>Focusing an element the Director has already chosen.
    ///
    /// The snapshot walk fetches elements in AutomationElementMode.None — cached
    /// data with no live reference — because that is what makes it fast enough to
    /// run after every step. Such an element can be READ but not operated, so
    /// focusing requires finding the live node again.
    ///
    /// Rather than slowing every observation down to keep live references around,
    /// the cost is paid only when an action actually needs one: a bounded walk of
    /// the target window comparing RuntimeIds. A snapshot happens constantly; a
    /// focus happens once.</summary>
    internal static class Focuser
    {
        public static string Focus(IntPtr hwnd, string runtimeId, int maxNodes)
        {
            if (hwnd == IntPtr.Zero) hwnd = Native.GetForegroundWindow();
            if (hwnd == IntPtr.Zero) return "no window to search";

            AutomationElement root;
            try { root = AutomationElement.FromHandle(hwnd); }
            catch (Exception e) { return "cannot attach to the window: " + e.Message; }
            if (root == null) return "the window exposes no automation element";

            int budget = maxNodes > 0 ? maxNodes : 1500;
            AutomationElement found = Find(root, runtimeId, ref budget);
            if (found == null)
            {
                // Not "focus failed" but "the element is gone" -- a meaningful
                // difference to the Director, which should re-observe rather than
                // retry an action against something that no longer exists.
                return budget <= 0
                    ? "gave up searching after " + maxNodes + " nodes"
                    : "no element with that id is in the window any more";
            }

            try
            {
                // SetFocus is the real thing: it moves keyboard focus without
                // activating, which is what distinguishes focusing from clicking.
                found.SetFocus();
                return "";
            }
            catch (Exception e)
            {
                return "the element refused focus: " + e.Message;
            }
        }

        /// <summary>Sets a control's value through the ValuePattern.
        ///
        /// This is the STRONGEST way to change text, and the reason is not speed. Typing
        /// sends characters through the keyboard layout, the IME, and whatever the
        /// application does with each keystroke -- autocomplete, validation, a dropdown
        /// that steals focus on the third letter. SetValue sets the value. There is no
        /// intermediate state for anything to react to and no layout to get wrong.
        ///
        /// Not every control supports it: read-only fields refuse, and many custom
        /// controls never implement the pattern. That is reported as a distinct failure
        /// so the caller can fall back deliberately rather than discover it by finding
        /// the field unchanged.</summary>
        public static string SetValue(IntPtr hwnd, string runtimeId, string value, int maxNodes)
        {
            AutomationElement found;
            string problem = Locate(hwnd, runtimeId, maxNodes, out found);
            if (problem.Length > 0) return problem;

            object pattern;
            if (!found.TryGetCurrentPattern(ValuePattern.Pattern, out pattern))
                return "unsupported: the control does not implement ValuePattern";

            var vp = (ValuePattern)pattern;
            if (vp.Current.IsReadOnly)
                return "unsupported: the control is read-only";

            try
            {
                vp.SetValue(value ?? "");
                return "";
            }
            catch (Exception e)
            {
                return "the control refused the value: " + e.Message;
            }
        }

        /// <summary>Reads a control's current value, so an append or an undo has
        /// something to compare against.</summary>
        public static string GetValue(IntPtr hwnd, string runtimeId, int maxNodes, out string value)
        {
            value = "";
            AutomationElement found;
            string problem = Locate(hwnd, runtimeId, maxNodes, out found);
            if (problem.Length > 0) return problem;

            object pattern;
            if (!found.TryGetCurrentPattern(ValuePattern.Pattern, out pattern))
                return "unsupported: the control does not implement ValuePattern";
            try
            {
                value = ((ValuePattern)pattern).Current.Value ?? "";
                return "";
            }
            catch (Exception e) { return "could not read the value: " + e.Message; }
        }

        /// <summary>The STRUCTURAL actions: asking the application to perform a semantic
        /// operation rather than synthesising the input that usually causes it.
        ///
        /// Every one follows the same contract, and the contract is what makes the
        /// Director's ladder honest:
        ///
        ///   - a control that does not implement the pattern fails with "unsupported:",
        ///     which the Director reads as "fall back" rather than as an outage;
        ///   - a control that implements it and refuses fails with its own message,
        ///     which is a fault and stops the action;
        ///   - nothing here substitutes a click. Deciding to click instead is the
        ///     Director's decision to make and to record, not the host's to make
        ///     silently.
        ///
        /// The "unsupported:" prefix is load-bearing — see marcoexec.unsupportedReason,
        /// which is the single place that distinguishes a refusal from a failure.</summary>
        public static string Invoke(IntPtr hwnd, string runtimeId, int maxNodes)
        {
            AutomationElement found;
            string problem = Locate(hwnd, runtimeId, maxNodes, out found);
            if (problem.Length > 0) return problem;

            object pattern;
            if (!found.TryGetCurrentPattern(InvokePattern.Pattern, out pattern))
                return "unsupported: the control does not implement InvokePattern";
            try
            {
                ((InvokePattern)pattern).Invoke();
                return "";
            }
            catch (Exception e) { return "the control refused to activate: " + e.Message; }
        }

        /// <summary>Expands or collapses a control through its ExpandCollapse pattern.
        ///
        /// A control already in the requested state is reported as OK rather than as an
        /// error: the user asked for it to be open, and it is. The Director short-circuits
        /// this case before calling, but a race between its observation and this call is
        /// ordinary, and failing it would turn a correct outcome into a retry.</summary>
        public static string ExpandCollapse(IntPtr hwnd, string runtimeId, bool expand,
                                            int maxNodes, out string state)
        {
            state = "";
            AutomationElement found;
            string problem = Locate(hwnd, runtimeId, maxNodes, out found);
            if (problem.Length > 0) return problem;

            object pattern;
            if (!found.TryGetCurrentPattern(ExpandCollapsePattern.Pattern, out pattern))
                return "unsupported: the control does not implement ExpandCollapsePattern";

            var ecp = (ExpandCollapsePattern)pattern;
            try
            {
                if (ecp.Current.ExpandCollapseState == ExpandCollapseState.LeafNode)
                    return "unsupported: the control has nothing to expand";
                if (expand) ecp.Expand(); else ecp.Collapse();
                state = ecp.Current.ExpandCollapseState.ToString().ToLowerInvariant();
                return "";
            }
            catch (Exception e)
            {
                return "the control refused to " + (expand ? "expand" : "collapse") + ": " + e.Message;
            }
        }

        /// <summary>Flips a two-state control, returning the state it landed in.
        ///
        /// The state comes back because the Director cannot otherwise tell a toggle that
        /// worked from one that was refused silently — and because Check and Uncheck are
        /// implemented as a toggle only after establishing the control was in the other
        /// state, so the resulting state is the proof.</summary>
        public static string Toggle(IntPtr hwnd, string runtimeId, int maxNodes, out string state)
        {
            state = "";
            AutomationElement found;
            string problem = Locate(hwnd, runtimeId, maxNodes, out found);
            if (problem.Length > 0) return problem;

            object pattern;
            if (!found.TryGetCurrentPattern(TogglePattern.Pattern, out pattern))
                return "unsupported: the control does not implement TogglePattern";
            var tp = (TogglePattern)pattern;
            try
            {
                tp.Toggle();
                state = tp.Current.ToggleState.ToString().ToLowerInvariant();
                return "";
            }
            catch (Exception e) { return "the control refused to toggle: " + e.Message; }
        }

        /// <summary>Selects a control among its siblings, or removes it from the
        /// selection.
        ///
        /// Deselect has no click equivalent, which is why the Director's ladder refuses
        /// rather than approximating it: a plain click SELECTS, and ctrl+click removing
        /// from a selection is a convention rather than a guarantee.</summary>
        public static string Select(IntPtr hwnd, string runtimeId, bool select, int maxNodes)
        {
            AutomationElement found;
            string problem = Locate(hwnd, runtimeId, maxNodes, out found);
            if (problem.Length > 0) return problem;

            object pattern;
            if (!found.TryGetCurrentPattern(SelectionItemPattern.Pattern, out pattern))
                return "unsupported: the control does not implement SelectionItemPattern";
            var sip = (SelectionItemPattern)pattern;
            try
            {
                if (select) sip.Select(); else sip.RemoveFromSelection();
                return "";
            }
            catch (Exception e)
            {
                return "the control refused to " + (select ? "select" : "deselect") + ": " + e.Message;
            }
        }

        /// <summary>Brings a control into view without moving the pointer.
        ///
        /// The only honest implementation of "scroll to it": wheel notches would scroll
        /// whichever container happens to be under the cursor, by an amount nobody
        /// chose, and would report success either way.</summary>
        public static string ScrollIntoView(IntPtr hwnd, string runtimeId, int maxNodes)
        {
            AutomationElement found;
            string problem = Locate(hwnd, runtimeId, maxNodes, out found);
            if (problem.Length > 0) return problem;

            object pattern;
            if (!found.TryGetCurrentPattern(ScrollItemPattern.Pattern, out pattern))
                return "unsupported: the control does not implement ScrollItemPattern";
            try
            {
                ((ScrollItemPattern)pattern).ScrollIntoView();
                return "";
            }
            catch (Exception e) { return "the control refused to scroll into view: " + e.Message; }
        }

        /// <summary>Finds the element, sharing the walk that Focus, SetValue and
        /// GetValue all need. One implementation so "the element is gone" means the same
        /// thing to all three.</summary>
        static string Locate(IntPtr hwnd, string runtimeId, int maxNodes, out AutomationElement found)
        {
            found = null;
            if (hwnd == IntPtr.Zero) hwnd = Native.GetForegroundWindow();
            if (hwnd == IntPtr.Zero) return "no window to search";

            AutomationElement root;
            try { root = AutomationElement.FromHandle(hwnd); }
            catch (Exception e) { return "cannot attach to the window: " + e.Message; }
            if (root == null) return "the window exposes no automation element";

            int budget = maxNodes > 0 ? maxNodes : 1500;
            found = Find(root, runtimeId, ref budget);
            if (found == null)
            {
                return budget <= 0
                    ? "gave up searching after " + maxNodes + " nodes"
                    : "no element with that id is in the window any more";
            }
            return "";
        }

        /// <summary>Resolves a control by the LABEL a person would call it, and returns its
        /// runtime id.
        ///
        /// This is what makes a saved play possible. A runtime id identifies a control in the
        /// tree as it stands right now and means nothing after the application redraws, so a
        /// play that held one would work until the first repaint and then fail obscurely. What
        /// a play holds instead is the word the person saw on the control, and this turns that
        /// word back into a control at the moment the play runs.
        ///
        /// Three outcomes, and the two refusals matter as much as the success:
        ///
        ///   - nothing matches      → "target_not_found:", the screen is not what the play
        ///                            expected;
        ///   - several match        → "target_ambiguous:", and it stops. Pressing the first of
        ///                            several controls sharing a name is a coin toss performed
        ///                            on somebody's computer, and it would be indistinguishable
        ///                            from working;
        ///   - exactly one matches  → that one.
        ///
        /// Matching is on the control's Name, trimmed and case-insensitive, and only over
        /// controls that are ENABLED and actually offer the Invoke pattern. A disabled twin or
        /// an offscreen duplicate is not a candidate, which is what keeps the ambiguous case
        /// rare enough to be honest about rather than tempting to guess around.</summary>
        public static string Resolve(IntPtr hwnd, string called, int maxNodes,
            out string runtimeId)
        {
            runtimeId = "";
            if (called == null || called.Trim().Length == 0) return "a control needs a name";
            if (hwnd == IntPtr.Zero) hwnd = Native.GetForegroundWindow();
            if (hwnd == IntPtr.Zero) return "no window to search";

            AutomationElement root;
            try { root = AutomationElement.FromHandle(hwnd); }
            catch (Exception e) { return "cannot attach to the window: " + e.Message; }
            if (root == null) return "the window exposes no automation element";

            var hits = new List<AutomationElement>();
            int budget = maxNodes > 0 ? maxNodes : 1500;
            Matching(root, called.Trim(), hits, ref budget);

            if (hits.Count == 0)
            {
                return budget <= 0
                    ? "target_not_found: gave up searching after " + maxNodes + " nodes"
                    : "target_not_found: nothing here is called \"" + called + "\"";
            }
            if (hits.Count > 1)
            {
                return "target_ambiguous: " + hits.Count + " controls here are called \"" +
                    called + "\", so I cannot tell which one you meant";
            }
            try { runtimeId = RuntimeIdString(hits[0].GetRuntimeId()); }
            catch (Exception e) { return "the control went away: " + e.Message; }
            if (runtimeId.Length == 0) return "target_not_found: the control has no id";
            return "";
        }

        /// <summary>Collects every enabled, invokable control with this label.</summary>
        static void Matching(AutomationElement el, string called, List<AutomationElement> hits,
            ref int budget)
        {
            if (el == null || budget <= 0) return;
            budget--;

            try
            {
                string name = el.Current.Name;
                if (name != null &&
                    string.Equals(name.Trim(), called, StringComparison.OrdinalIgnoreCase) &&
                    el.Current.IsEnabled && Invokable(el))
                {
                    hits.Add(el);
                }
            }
            catch { /* element vanished mid-walk */ }

            var walker = TreeWalker.ControlViewWalker;
            AutomationElement child = null;
            try { child = walker.GetFirstChild(el); } catch { return; }
            while (child != null && budget > 0)
            {
                Matching(child, called, hits, ref budget);
                try { child = walker.GetNextSibling(child); } catch { return; }
            }
        }

        /// <summary>Whether this control can be pressed at all.
        ///
        /// Part of the match rather than a check afterwards: a label shared by a button and a
        /// static caption is not ambiguous, because only one of them is a thing you can press.
        /// Selection items count — "make this the selection" is how a list row is activated,
        /// and Settings' navigation list is exactly that.</summary>
        static bool Invokable(AutomationElement el)
        {
            object p;
            if (el.TryGetCurrentPattern(InvokePattern.Pattern, out p)) return true;
            if (el.TryGetCurrentPattern(SelectionItemPattern.Pattern, out p)) return true;
            return false;
        }

        static AutomationElement Find(AutomationElement el, string runtimeId, ref int budget)
        {
            if (el == null || budget <= 0) return null;
            budget--;

            try
            {
                int[] rid = el.GetRuntimeId();
                if (rid != null && RuntimeIdString(rid) == runtimeId) return el;
            }
            catch { /* element vanished mid-walk */ }

            var walker = TreeWalker.ControlViewWalker;
            AutomationElement child = null;
            try { child = walker.GetFirstChild(el); } catch { return null; }
            while (child != null && budget > 0)
            {
                var hit = Find(child, runtimeId, ref budget);
                if (hit != null) return hit;
                try { child = walker.GetNextSibling(child); } catch { return null; }
            }
            return null;
        }

        public static string RuntimeIdString(int[] arr)
        {
            if (arr == null || arr.Length == 0) return "";
            var sb = new StringBuilder("uia:");
            for (int i = 0; i < arr.Length; i++)
            {
                if (i > 0) sb.Append('.');
                sb.Append(arr[i]);
            }
            return sb.ToString();
        }
    }

    internal static class Walker
    {
        // Properties fetched in the bulk cache request. Adding one here costs nothing
        // per element; reading one that isn't here costs a cross-process round trip.
        static readonly AutomationProperty[] CachedProps =
        {
            AutomationElement.NameProperty,
            AutomationElement.ControlTypeProperty,
            AutomationElement.AutomationIdProperty,
            AutomationElement.ClassNameProperty,
            AutomationElement.BoundingRectangleProperty,
            AutomationElement.IsEnabledProperty,
            AutomationElement.IsOffscreenProperty,
            AutomationElement.HasKeyboardFocusProperty,
            AutomationElement.RuntimeIdProperty,
            AutomationElement.HelpTextProperty,
            ValuePattern.ValueProperty,
            SelectionItemPattern.IsSelectedProperty,
            TogglePattern.ToggleStateProperty,
            ExpandCollapsePattern.ExpandCollapseStateProperty,

            // The pattern-availability properties, which are what the Director's
            // capability ladder reads.
            //
            // They MUST be in this list. The walk fetches elements in
            // AutomationElementMode.None — cached data with no live reference — so a
            // property that was not requested reads as NotSupported rather than as its
            // real value. Left out, every control on the desktop reported implementing
            // nothing, and the ladder fell every verb through to a click while looking
            // like it was making an informed choice. Found by snapshotting Notepad and
            // seeing seventeen buttons with no Invoke pattern between them.
            AutomationElement.IsInvokePatternAvailableProperty,
            AutomationElement.IsExpandCollapsePatternAvailableProperty,
            AutomationElement.IsTogglePatternAvailableProperty,
            AutomationElement.IsSelectionItemPatternAvailableProperty,
            AutomationElement.IsValuePatternAvailableProperty,
            AutomationElement.IsScrollItemPatternAvailableProperty,
            AutomationElement.IsScrollPatternAvailableProperty,
            AutomationElement.IsWindowPatternAvailableProperty,
        };

        public static Snapshot Snapshot(WalkOptions opts)
        {
            var sw = Stopwatch.StartNew();
            var snap = new Snapshot();

            IntPtr hwnd = opts.Window != IntPtr.Zero ? opts.Window : Native.GetForegroundWindow();
            if (hwnd == IntPtr.Zero)
            {
                snap.Partial = true;
                snap.Reason = "no foreground window";
                return snap;
            }

            snap.WindowId = "hwnd:" + hwnd.ToInt64().ToString();
            snap.WindowTitle = Native.WindowTitle(hwnd);
            snap.App = Native.ExeName(hwnd);
            snap.ProcessId = Native.ProcessId(hwnd);
            snap.Minimized = Native.IsIconic(hwnd);
            snap.Maximized = Native.IsZoomed(hwnd);
            snap.WindowVisible = Native.IsWindowVisible(hwnd);
            Native.RECT r;
            if (Native.GetWindowRect(hwnd, out r))
            {
                snap.WX = r.Left;
                snap.WY = r.Top;
                snap.WW = r.Right - r.Left;
                snap.WH = r.Bottom - r.Top;
            }

            AutomationElement root;
            try { root = AutomationElement.FromHandle(hwnd); }
            catch (Exception e)
            {
                snap.Partial = true;
                snap.Reason = "cannot attach to the window: " + e.Message;
                return snap;
            }
            if (root == null)
            {
                snap.Partial = true;
                snap.Reason = "the window exposes no automation element";
                return snap;
            }

            // Preferred path: one bulk fetch of the whole subtree.
            try
            {
                var cr = new CacheRequest();
                foreach (var p in CachedProps) cr.Add(p);
                cr.TreeScope = TreeScope.Element | TreeScope.Subtree;
                cr.TreeFilter = Automation.ControlViewCondition;
                cr.AutomationElementMode = AutomationElementMode.None; // cached data only — cheaper

                AutomationElement cached;
                using (cr.Activate()) { cached = root.GetUpdatedCache(cr); }
                EmitCached(cached, null, 0, snap, opts, sw);
                AttachShellResource(snap, hwnd);
                snap.ElapsedMs = sw.ElapsedMilliseconds;
                return snap;
            }
            catch (Exception e)
            {
                // Some applications refuse a subtree cache request (custom providers,
                // certain Electron builds). Fall back to a live walk: slower, but a
                // degraded snapshot beats none, and the caller is told which it got.
                snap.Nodes.Clear();
                snap.Reason = "bulk cache unavailable, used a live walk: " + e.Message;
                try
                {
                    EmitLive(root, null, 0, snap, opts, sw);
                    AttachShellResource(snap, hwnd);
                }
                catch (Exception e2)
                {
                    snap.Partial = true;
                    snap.Reason = "walk failed: " + e2.Message;
                }
                snap.ElapsedMs = sw.ElapsedMilliseconds;
                return snap;
            }
        }

        // AttachShellResource gives the selected Explorer item the path the shell knows.
        //
        // The correlation, and every step of it is a chance to refuse:
        //
        //  1. The window's shell view must report a filesystem folder and EXACTLY ONE
        //     selected item with a path inside it. (Shell.cs does all of that.)
        //  2. Exactly one node in the walk must be the selected item. Several selected
        //     nodes means UI Automation and the shell disagree about the selection, and a
        //     disagreement about which object is meant is not something to resolve by
        //     preference.
        //  3. Their names must MATCH. This is the check that makes the result an
        //     identification rather than an assumption: the shell says "the selected item
        //     is Alpha.txt at this path", the tree says "this node is the selected one and
        //     it is called Alpha.txt", and only when both are true is the path attached to
        //     that node.
        //
        // Hidden file extensions are why the comparison allows the caption to be the name
        // without its extension: with extensions hidden the tree shows "Alpha" for
        // "Alpha.txt". That is a widening of what MATCHES, never of what is attached — the
        // path always comes from the shell, and a caption that matches nothing gets
        // nothing.
        static void AttachShellResource(Snapshot snap, IntPtr hwnd)
        {
            ShellView view;
            try { view = ShellItems.ForWindow(hwnd, snap.App); }
            catch (Exception e)
            {
                snap.Shell = new ShellView { Refusal = "shell lookup failed: " + e.Message };
                return;
            }
            snap.Shell = view;
            if (view.Selected == null) return;

            // Among the SELECTED nodes, the ones the shell's item is called.
            //
            // Not "exactly one node is selected": Explorer marks several things selected
            // at once — the list item, the tab it sits in, the navigation-pane entry for
            // its folder — and a rule that demanded a single selected node refused every
            // real Explorer window. That was found by snapshotting one: the shell said
            // one item, the tree said four.
            //
            // What must be unique is the MATCH. The shell names one item; exactly one
            // selected node may be called that; and then the two agree about which object
            // is meant. Two selected nodes with the same name is a genuine ambiguity and
            // is refused.
            Node match = null;
            int selectedNodes = 0, matches = 0;
            foreach (var n in snap.Nodes)
            {
                if (!n.Selected) continue;
                selectedNodes++;
                if (!NameMatches(n.Label, view.Selected.DisplayName)) continue;
                matches++;
                match = n;
            }
            if (selectedNodes == 0)
            {
                view.Refusal = "the shell reports \"" + view.Selected.DisplayName +
                    "\" as selected and the tree reports nothing as selected";
                view.Selected = null;
                return;
            }
            if (matches == 0)
            {
                view.Refusal = "the selected item is called \"" + view.Selected.DisplayName +
                    "\" in the shell and no selected control in the tree is called that";
                view.Selected = null;
                return;
            }
            if (matches > 1)
            {
                view.Refusal = matches + " selected controls are called \"" +
                    view.Selected.DisplayName + "\", so which one the shell means cannot " +
                    "be established";
                view.Selected = null;
                return;
            }
            match.Resource = view.Selected;
        }

        // NameMatches compares a control's caption with the shell's name for an item.
        //
        // Equal, or equal once the extension is dropped — which is what Explorer shows when
        // "hide extensions for known file types" is on. Nothing looser: a prefix or
        // substring rule would match "Alpha.txt" against "Alpha (copy).txt".
        static bool NameMatches(string caption, string shellName)
        {
            if (caption == null || shellName == null) return false;
            caption = caption.Trim();
            shellName = shellName.Trim();
            if (caption.Length == 0 || shellName.Length == 0) return false;
            if (string.Equals(caption, shellName, StringComparison.OrdinalIgnoreCase)) return true;

            int dot = shellName.LastIndexOf('.');
            if (dot > 0)
            {
                string stem = shellName.Substring(0, dot);
                if (string.Equals(caption, stem, StringComparison.OrdinalIgnoreCase)) return true;
            }
            return false;
        }

        static bool Bounded(Snapshot snap, WalkOptions opts, Stopwatch sw, int depth)
        {
            if (snap.Nodes.Count >= opts.MaxNodes)
            {
                snap.Partial = true;
                if (snap.Reason == "") snap.Reason = "node limit (" + opts.MaxNodes + ") reached";
                return true;
            }
            if (depth > opts.MaxDepth)
            {
                snap.Partial = true;
                if (snap.Reason == "") snap.Reason = "depth limit (" + opts.MaxDepth + ") reached";
                return true;
            }
            if (sw.ElapsedMilliseconds > opts.TimeoutMs)
            {
                snap.Partial = true;
                if (snap.Reason == "") snap.Reason = "timed out after " + opts.TimeoutMs + "ms";
                return true;
            }
            return false;
        }

        static void EmitCached(AutomationElement el, string parentId, int depth,
                               Snapshot snap, WalkOptions opts, Stopwatch sw)
        {
            if (el == null || Bounded(snap, opts, sw, depth)) return;

            Node n = FromCached(el, parentId, depth);
            if (n == null) return;
            snap.Nodes.Add(n);

            AutomationElementCollection kids;
            try { kids = el.CachedChildren; }
            catch { return; }
            if (kids == null) return;
            for (int i = 0; i < kids.Count; i++)
            {
                if (Bounded(snap, opts, sw, depth + 1)) return;
                EmitCached(kids[i], n.Id, depth + 1, snap, opts, sw);
            }
        }

        static void EmitLive(AutomationElement el, string parentId, int depth,
                             Snapshot snap, WalkOptions opts, Stopwatch sw)
        {
            if (el == null || Bounded(snap, opts, sw, depth)) return;

            Node n = FromLive(el, parentId, depth);
            if (n == null) return;
            snap.Nodes.Add(n);

            var walker = TreeWalker.ControlViewWalker;
            AutomationElement child = null;
            try { child = walker.GetFirstChild(el); } catch { return; }
            while (child != null)
            {
                if (Bounded(snap, opts, sw, depth + 1)) return;
                EmitLive(child, n.Id, depth + 1, snap, opts, sw);
                try { child = walker.GetNextSibling(child); } catch { return; }
            }
        }

        // ── property reads ───────────────────────────────────────────────────────
        // Every one is guarded. An element that disappears between the cache fetch and
        // the read throws ElementNotAvailableException, and one vanished tooltip must
        // not abort a whole snapshot.

        static object Cached(AutomationElement el, AutomationProperty p)
        {
            try
            {
                object v = el.GetCachedPropertyValue(p, true);
                return v == AutomationElement.NotSupported ? null : v;
            }
            catch { return null; }
        }

        static object Live(AutomationElement el, AutomationProperty p)
        {
            try
            {
                object v = el.GetCurrentPropertyValue(p, true);
                return v == AutomationElement.NotSupported ? null : v;
            }
            catch { return null; }
        }

        static Node FromCached(AutomationElement el, string parentId, int depth)
        {
            return Build(el, parentId, depth, Cached);
        }

        static Node FromLive(AutomationElement el, string parentId, int depth)
        {
            return Build(el, parentId, depth, Live);
        }

        /// <summary>The patterns a control implements, named the way the Director's
        /// ladder names them.
        ///
        /// Read through the Is*PatternAvailable PROPERTIES rather than by calling
        /// TryGetCurrentPattern, because the snapshot walk fetches elements in
        /// AutomationElementMode.None — cached data with no live reference — and asking
        /// such an element for a live pattern throws. The availability properties are
        /// cacheable and answer the same question.
        ///
        /// Names are lower-cased and provider-neutral ("expandcollapse", not
        /// "ExpandCollapsePattern"), so a future macOS AX provider reporting the same
        /// capability lands in the same rung without a second vocabulary.</summary>
        static List<string> PatternsOf(AutomationElement el,
                                       Func<AutomationElement, AutomationProperty, object> read)
        {
            var found = new List<string>();
            Action<AutomationProperty, string> check = (prop, name) =>
            {
                if (AsBool(read(el, prop), false)) found.Add(name);
            };
            check(AutomationElement.IsInvokePatternAvailableProperty, "invoke");
            check(AutomationElement.IsExpandCollapsePatternAvailableProperty, "expandcollapse");
            check(AutomationElement.IsTogglePatternAvailableProperty, "toggle");
            check(AutomationElement.IsSelectionItemPatternAvailableProperty, "selectionitem");
            check(AutomationElement.IsValuePatternAvailableProperty, "value");
            check(AutomationElement.IsScrollItemPatternAvailableProperty, "scrollitem");
            check(AutomationElement.IsScrollPatternAvailableProperty, "scroll");
            check(AutomationElement.IsWindowPatternAvailableProperty, "window");
            return found;
        }

        static Node Build(AutomationElement el, string parentId, int depth,
                          Func<AutomationElement, AutomationProperty, object> read)
        {
            var n = new Node { ParentId = parentId, Depth = depth };

            n.Id = RuntimeIdOf(read(el, AutomationElement.RuntimeIdProperty));
            if (n.Id == "") return null; // no identity — unusable as a stable target

            var ct = read(el, AutomationElement.ControlTypeProperty) as ControlType;
            n.ControlType = ct != null ? ct.ProgrammaticName : "";
            n.Role = RoleOf(ct);

            n.Label = AsString(read(el, AutomationElement.NameProperty));
            n.Description = AsString(read(el, AutomationElement.HelpTextProperty));
            n.AutomationId = AsString(read(el, AutomationElement.AutomationIdProperty));
            n.ClassName = AsString(read(el, AutomationElement.ClassNameProperty));
            n.Value = AsString(read(el, ValuePattern.ValueProperty));

            n.Enabled = AsBool(read(el, AutomationElement.IsEnabledProperty), true);
            n.Offscreen = AsBool(read(el, AutomationElement.IsOffscreenProperty), false);
            n.Focused = AsBool(read(el, AutomationElement.HasKeyboardFocusProperty), false);
            n.Selected = AsBool(read(el, SelectionItemPattern.IsSelectedProperty), false);

            // A toggle's state is its VALUE, and the Director needs it to verify "the
            // checkbox is now ticked" without re-reading the tree a second way.
            object toggle = read(el, TogglePattern.ToggleStateProperty);
            if (toggle is ToggleState)
            {
                var ts = (ToggleState)toggle;
                if (n.Value == "") n.Value = toggle.ToString().ToLowerInvariant();
                // Indeterminate stays EMPTY rather than becoming false: a tri-state box
                // that is neither on nor off is exactly the case where "uncheck it"
                // must act rather than conclude it is already done.
                if (ts == ToggleState.On) n.Checked = "true";
                else if (ts == ToggleState.Off) n.Checked = "false";
            }

            object expand = read(el, ExpandCollapsePattern.ExpandCollapseStateProperty);
            if (expand is ExpandCollapseState)
            {
                var es = (ExpandCollapseState)expand;
                // LeafNode is left empty for the same reason: a node with nothing to show
                // is not "collapsed", and treating it as such would make "expand it"
                // report success having done nothing.
                if (es == ExpandCollapseState.Expanded) n.Expanded = "true";
                else if (es == ExpandCollapseState.Collapsed) n.Expanded = "false";
            }

            n.Patterns = PatternsOf(el, read);

            var rect = read(el, AutomationElement.BoundingRectangleProperty);
            if (rect is System.Windows.Rect)
            {
                var r = (System.Windows.Rect)rect;
                // UIA reports an empty rect as (0,0,0,0) or as infinities for an
                // element with no on-screen presence. Both must come through as
                // "unknown bounds" rather than as the desktop origin, which is a real
                // clickable point.
                if (!double.IsInfinity(r.X) && !double.IsInfinity(r.Y) &&
                    !double.IsNaN(r.X) && !double.IsNaN(r.Y) &&
                    r.Width > 0 && r.Height > 0)
                {
                    n.X = (int)Math.Round(r.X);
                    n.Y = (int)Math.Round(r.Y);
                    n.W = (int)Math.Round(r.Width);
                    n.H = (int)Math.Round(r.Height);
                }
            }

            // Visible is derived, not reported: UIA has no "visible" property. An
            // element is treated as visible when it is on screen and has area.
            n.Visible = !n.Offscreen && n.W > 0 && n.H > 0;
            return n;
        }

        static string AsString(object v) { return v == null ? "" : v.ToString(); }

        static bool AsBool(object v, bool fallback)
        {
            if (v is bool) return (bool)v;
            return fallback;
        }

        /// <summary>RuntimeId is UIA's own identity for an element — an int array that
        /// is stable for as long as the element lives. It is the single best signal the
        /// Director's identity matcher has: when it is unchanged between snapshots,
        /// continuity is settled outright and nothing needs to be inferred from labels
        /// or geometry.</summary>
        static string RuntimeIdOf(object v)
        {
            var arr = v as int[];
            if (arr == null || arr.Length == 0) return "";
            var sb = new StringBuilder("uia:");
            for (int i = 0; i < arr.Length; i++)
            {
                if (i > 0) sb.Append('.');
                sb.Append(arr[i]);
            }
            return sb.ToString();
        }

        /// <summary>Maps a UIA control type onto the Director's normalised role
        /// vocabulary. Sources speak different dialects (UIA control types, ARIA roles,
        /// detector class labels); mapping at the edge means the Director works in one
        /// vocabulary and an unrecognised type degrades to "unknown" rather than leaking
        /// a provider-specific string into planning.</summary>
        static string RoleOf(ControlType ct)
        {
            if (ct == null) return "unknown";
            if (ct == ControlType.Button) return "button";
            if (ct == ControlType.SplitButton) return "button";
            if (ct == ControlType.Edit) return "text_field";
            if (ct == ControlType.Document) return "text_field";
            if (ct == ControlType.CheckBox) return "checkbox";
            if (ct == ControlType.RadioButton) return "radio";
            if (ct == ControlType.ComboBox) return "combo_box";
            if (ct == ControlType.List) return "list";
            if (ct == ControlType.ListItem) return "list_item";
            if (ct == ControlType.Menu) return "menu";
            if (ct == ControlType.MenuBar) return "menu";
            if (ct == ControlType.MenuItem) return "menu_item";
            if (ct == ControlType.Tab) return "tab_list";
            if (ct == ControlType.TabItem) return "tab";
            if (ct == ControlType.Hyperlink) return "link";
            if (ct == ControlType.Image) return "image";
            if (ct == ControlType.Text) return "text";
            if (ct == ControlType.Window) return "window";
            if (ct == ControlType.Pane) return "pane";
            if (ct == ControlType.Group) return "group";
            if (ct == ControlType.ToolBar) return "toolbar";
            if (ct == ControlType.TitleBar) return "title_bar";
            if (ct == ControlType.ScrollBar) return "scroll_bar";
            if (ct == ControlType.Slider) return "slider";
            if (ct == ControlType.ProgressBar) return "progress_bar";
            if (ct == ControlType.Table) return "table";
            if (ct == ControlType.DataGrid) return "table";
            if (ct == ControlType.DataItem) return "row";
            if (ct == ControlType.HeaderItem) return "cell";
            if (ct == ControlType.Tree) return "tree";
            if (ct == ControlType.TreeItem) return "tree_item";
            return "unknown";
        }
    }
}
