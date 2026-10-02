// Command fixture is the deterministic desktop Marco is tested against.
//
// # Why a fixture application exists at all
//
// Every real-desktop acceptance this repository has run was aimed at somebody else's software —
// Windows Settings, File Explorer, Chrome, Discord, VS Code. That made every result two claims at
// once: something about Marco, and something about an application that updates on its own
// schedule. Nine acceptance scripts carry workarounds for the second claim, and the audit of the
// perception work found three separate investigations whose conclusions were really about one
// version of one Microsoft page.
//
// This window is ours. It has a title that does not change, controls whose names do not change,
// a layout that does not reflow, and no update channel. When an acceptance fails against it, the
// finding is about Marco.
//
// # Why C# and WinForms
//
// Because the accessibility has to be LEGITIMATE. A hand-rolled Win32 window would need its
// accessible names inferred by the MSAA bridge from label adjacency, which is exactly the kind of
// platform-specific behaviour a fixture must not be testing; and a custom UI Automation provider
// written for Marco would be a fixture that answers Marco's questions rather than an application
// Marco has to read. WinForms controls are ordinary Win32 controls underneath, surfaced through
// the standard MSAA-to-UIA bridge, and `AccessibleName` is the property every real application
// sets. Marco sees this the way it sees anything else.
//
// It also costs no new build dependency: the same `csc.exe` that already builds plugins/uia, with
// no SDK, no NuGet and no restore. See build.ps1.
//
// # The oracle, and the rule about it
//
// Every change writes fixture-state.json: what the fields hold, what is checked, what is
// selected, what the status says, how many times each button was pressed. That file is the
// harness's INDEPENDENT GROUND TRUTH, and the whole value of it rests on one rule:
//
//	MARCO MUST NEVER READ IT.
//
// If Marco could read it, an acceptance proving "Marco clicked Save" would be proving that Marco
// can read a file. The harness compares Marco's own account of what happened against this
// afterwards, which catches the failure no single channel can see: Marco being confident and
// wrong. internal/acceptance holds a boundary test over that rule.
//
// # Deliberate misbehaviour
//
//	--stubborn   Save is present, enabled, and does nothing.
//
// There is no other way to construct the case that matters most. "Marco pressed a control and
// believed the outcome occurred when it did not" cannot be produced by breaking Marco — it has to
// be produced by an application that accepts a press and declines to act, which is a thing real
// software does constantly (a disabled-looking-but-enabled button, a validation failure, a
// swallowed exception). In that mode Marco's own verification must fail. If it passes, Marco is
// claiming an outcome it did not get.
//
// Build:  powershell -File plugins/fixture/build.ps1
// Run:    plugins/fixture/fixture.exe --state <path> [--stubborn] [--view second]

using System;
using System.Drawing;
using System.Globalization;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using System.Windows.Forms;

namespace MarcoFixture
{
    internal static class Program
    {
        [STAThread]
        private static int Main(string[] args)
        {
            string state = null;
            bool stubborn = false;
            for (int i = 0; i < args.Length; i++)
            {
                switch (args[i])
                {
                    case "--state":
                        if (i + 1 < args.Length) { state = args[++i]; }
                        break;
                    case "--stubborn":
                        stubborn = true;
                        break;
                    default:
                        Console.Error.WriteLine("fixture: unknown argument " + args[i]);
                        return 2;
                }
            }
            if (string.IsNullOrEmpty(state))
            {
                Console.Error.WriteLine("usage: fixture.exe --state <path> [--stubborn]");
                return 2;
            }

            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);
            Application.Run(new FixtureForm(state, stubborn));
            return 0;
        }
    }

    /// <summary>The one window. Deterministic in title, layout, control names and position.</summary>
    internal sealed class FixtureForm : Form
    {
        private readonly string statePath;
        private readonly bool stubborn;

        private readonly TextBox name = new TextBox();
        private readonly TextBox notes = new TextBox();
        private readonly CheckBox remember = new CheckBox();
        private readonly ListBox items = new ListBox();
        private readonly Button save = new Button();
        private readonly Button cancel = new Button();
        private readonly Label status = new Label();
        private readonly Button details = new Button();
        private readonly Panel secondView = new Panel();
        private readonly Label secondText = new Label();
        // A control that exists, is visible, is named, and will not respond. Enabled=false is the
        // honest version of the stubborn button: an application saying "not now" in the one way
        // the accessibility layer reports truthfully.
        private readonly Button deleteBtn = new Button();
        // Something to expand. SemanticExpand and verifyExpansion have never run against a real
        // desktop, and a tree is the ordinary control they were written for.
        private readonly TreeView tree = new TreeView();

        private int savePresses;
        private int cancelPresses;
        private int detailsPresses;
        private int expandEvents;
        private int collapseEvents;
        private bool documentsExpanded;

        internal FixtureForm(string statePath, bool stubborn)
        {
            this.statePath = statePath;
            this.stubborn = stubborn;

            // THE TITLE IS FIXED AND UNIQUE. Marco finds its application by name, and a title
            // carrying a document, a count or a dirty marker would make "which window" a moving
            // question — which is a thing to test deliberately, elsewhere, not by accident here.
            Text = "Marco Fixture";
            FormBorderStyle = FormBorderStyle.FixedDialog;
            MaximizeBox = false;
            MinimizeBox = false;
            StartPosition = FormStartPosition.Manual;
            // A DETERMINISTIC POSITION AND SIZE, so normalised geometry is stable across runs and
            // a Place signature cannot differ because a window manager put the window somewhere
            // else. Not centred: "centre" depends on the monitor.
            Location = new Point(120, 120);
            ClientSize = new Size(460, 400);
            AccessibleName = "Marco Fixture";

            // ── the first view ────────────────────────────────────────────────
            //
            // EVERY CONTROL CARRIES AN AccessibleName, and each is the word a person would use
            // for it. This is the ordinary WinForms accessibility property, not a Marco hook:
            // what Marco reads here is what it reads from any application whose author set it.
            var nameLabel = Lbl("Name", 16, 20);
            name.SetBounds(90, 16, 200, 24);
            name.AccessibleName = "Name";
            name.TabIndex = 0;
            name.TextChanged += delegate { Write(); };

            var notesLabel = Lbl("Notes", 16, 56);
            notes.SetBounds(90, 52, 200, 24);
            notes.AccessibleName = "Notes";
            notes.TabIndex = 1;
            notes.TextChanged += delegate { Write(); };

            remember.SetBounds(90, 88, 160, 24);
            remember.Text = "Remember me";
            remember.AccessibleName = "Remember me";
            remember.TabIndex = 2;
            remember.CheckedChanged += delegate { Write(); };

            var itemsLabel = Lbl("Items", 16, 124);
            items.SetBounds(90, 120, 200, 76);
            items.AccessibleName = "Items";
            items.TabIndex = 3;
            items.Items.AddRange(new object[] { "Alpha", "Beta", "Gamma" });
            items.SelectedIndexChanged += delegate { Write(); };

            save.SetBounds(90, 212, 90, 28);
            save.Text = "Save";
            save.AccessibleName = "Save";
            save.TabIndex = 4;
            save.Click += OnSave;

            cancel.SetBounds(190, 212, 90, 28);
            cancel.Text = "Cancel";
            cancel.AccessibleName = "Cancel";
            cancel.TabIndex = 5;
            cancel.Click += OnCancel;

            details.SetBounds(290, 212, 90, 28);
            details.Text = "Details";
            details.AccessibleName = "Details";
            details.TabIndex = 6;
            details.Click += OnDetails;

            status.SetBounds(16, 256, 420, 24);
            status.Text = "Status: ready";
            // NO AccessibleName HERE, DELIBERATELY.
            //
            // An AccessibleName REPLACES the displayed text in the accessibility tree, so setting
            // one made Marco read a control called "Status" whose content it could never see —
            // and the whole point of this label is that Marco must be able to read the outcome off
            // the screen. Measured: with a name set, `inspect` reported `text "Status"` and the
            // word "Saved" was nowhere in the tree.
            //
            // Left unset, the accessible name defaults to the text, which is both what a real
            // application does for a status line and the only way an acceptance can verify an
            // outcome through perception rather than through the oracle.

            // ── the second view ───────────────────────────────────────────────
            //
            // A PART OF ONE WINDOW HOLDING A DIFFERENT KIND OF THING, which is the case Marco's
            // two-level screen identity exists for: the same application surface, a different
            // place inside it. Hidden until Details is pressed, so a run can produce a genuine
            // within-a-window change rather than only a value change.
            secondView.SetBounds(16, 288, 420, 90);
            secondView.Visible = false;
            secondView.AccessibleName = "Details panel";
            secondText.SetBounds(8, 8, 400, 24);
            secondText.Text = "Nothing selected";
            // Unnamed for the same reason the status line is: its content is the thing to read.
            var back = new Button();
            back.SetBounds(8, 40, 90, 28);
            back.Text = "Back";
            back.AccessibleName = "Back";
            back.Click += delegate
            {
                secondView.Visible = false;
                status.Text = "Status: ready";
                Write();
            };
            secondView.Controls.Add(secondText);
            secondView.Controls.Add(back);

            // ── a control that will not respond ───────────────────────────────
            //
            // Disabled, and that is the whole point: Marco must refuse rather than report success,
            // and the refusal has to come from the control's own reported state rather than from
            // anything about this application.
            deleteBtn.SetBounds(16, 248, 90, 28);
            deleteBtn.Text = "Delete";
            deleteBtn.AccessibleName = "Delete";
            deleteBtn.Enabled = false;
            deleteBtn.Click += delegate
            {
                // Unreachable while Enabled is false. Present so that a run which DID reach it
                // leaves a mark in the oracle rather than passing silently.
                status.Text = "Status: Deleted";
                Write();
            };

            // ── something to expand ───────────────────────────────────────────
            tree.SetBounds(16, 288, 200, 100);
            tree.AccessibleName = "Folders";
            var root = new TreeNode("Documents");
            root.Nodes.Add(new TreeNode("Reports"));
            root.Nodes.Add(new TreeNode("Invoices"));
            tree.Nodes.Add(root);
            tree.Nodes.Add(new TreeNode("Pictures"));
            // STATE FROM THE EVENTS, NOT FROM TreeNode.IsExpanded.
            //
            // Measured: a UIA-driven collapse left `IsExpanded` reporting true while UIA said
            // Collapsed AND the child nodes had vanished from the accessibility tree. Two
            // independent signals said the node had collapsed and the managed property said it
            // had not, so the ORACLE was the unreliable witness and Marco was right.
            //
            // That is the one failure mode an oracle may not have. Its whole value is being an
            // independent account of the truth, and an oracle that reports a stale property turns
            // a correct Marco into a false failure — which is exactly as damaging as the false
            // success this harness was built to catch, and harder to notice because it looks like
            // diligence.
            //
            // The events are the application telling us what it did. Counted as well as
            // recorded, so "it never happened" and "it happened and went back" cannot read alike.
            tree.AfterExpand += delegate { documentsExpanded = true; expandEvents++; Write(); };
            tree.AfterCollapse += delegate { documentsExpanded = false; collapseEvents++; Write(); };

            var treeLabel = Lbl("Folders", 16, 268);

            status.SetBounds(16, 400, 420, 24);
            secondView.SetBounds(228, 288, 208, 100);
            ClientSize = new Size(460, 440);

            Controls.AddRange(new Control[]
            {
                nameLabel, name, notesLabel, notes, remember,
                itemsLabel, items, save, cancel, details, deleteBtn,
                treeLabel, tree, status, secondView,
            });

            Shown += delegate { Write(); };

            // ── the raise channel ─────────────────────────────────────────────
            //
            // # Why the fixture raises ITSELF
            //
            // Windows refuses SetForegroundWindow to a process that does not already own the
            // foreground — the foreground lock, and it is not negotiable. Measured on this
            // machine: the call from outside returned False and the foreground did not move. The
            // existing Explorer harness works around it by re-opening the shell on the folder,
            // which raises the window as a side effect; a fixture has no such back door.
            //
            // So the harness ASKS, by creating `<state>.raise`, and the fixture does the raising
            // from inside its own process where the rights exist.
            //
            // # Why this is arrangement and not cheating
            //
            // It is the same split the Explorer harness already draws: the harness may put the
            // window in front, and Marco's own foregroundGuard is what CHECKS that it is there at
            // the instant of execution. Bringing a window forward is setup — the user action under
            // test is pressing Save, and nothing here presses anything.
            //
            // Deliberately NOT TopMost for the whole run: a window floating permanently above
            // somebody's screen is a worse neighbour than one that takes the foreground when asked
            // and gives it back when the test ends.
            var raiser = new Timer();
            raiser.Interval = 150;
            raiser.Tick += delegate { Raise(statePath + ".raise"); };
            raiser.Start();
        }

        [DllImport("user32.dll")]
        private static extern bool SetForegroundWindow(IntPtr h);

        [DllImport("user32.dll")]
        private static extern IntPtr GetForegroundWindow();

        /// <summary>Comes to the front when the harness asks, and reports that it did.</summary>
        ///
        /// The acknowledgement matters: the harness has to be able to WAIT for the foreground
        /// rather than sleeping a guessed interval, and a sleep is how a harness becomes flaky on
        /// a loaded machine. It writes `<state>.raised` so the wait has something to observe.
        private void Raise(string request)
        {
            try
            {
                if (!File.Exists(request)) { return; }
                File.Delete(request);
            }
            catch (IOException)
            {
                return;
            }

            if (WindowState == FormWindowState.Minimized) { WindowState = FormWindowState.Normal; }
            // TopMost briefly rather than permanently: it lifts the window above whatever is in
            // front without leaving it pinned there, and Activate alone loses to a full-screen
            // application.
            TopMost = true;
            Activate();
            BringToFront();
            SetForegroundWindow(Handle);
            TopMost = false;
            Activate();

            // AND THE CASE WHERE NOTHING HOLDS THE FOREGROUND AT ALL.
            //
            // # Why this fallback exists, measured
            //
            // The dance above works when an ordinary window is in front: it takes the foreground
            // from it. It does NOT work when the foreground is empty — `SetForegroundWindow`
            // returns without effect and `marco active` keeps reporting nothing.
            //
            // That state is one the harness CREATES. Killing a fixture that held the foreground
            // leaves nothing holding it, so the next run could not start: the fixture raised
            // itself, acknowledged the request, and stayed behind. An acceptance system that can
            // only run once without somebody clicking a window is not an unattended one, which is
            // the whole point of it.
            //
            // A restore from minimised activates from inside the owning process, which is where
            // the rights live. It is the fixture doing it to its own window — no other window is
            // touched, minimised or rearranged.
            if (GetForegroundWindow() != Handle)
            {
                WindowState = FormWindowState.Minimized;
                WindowState = FormWindowState.Normal;
                Activate();
                SetForegroundWindow(Handle);
            }

            try
            {
                File.WriteAllText(request.Substring(0, request.Length - ".raise".Length)
                    + ".raised", DateTime.UtcNow.ToString("o", CultureInfo.InvariantCulture));
            }
            catch (IOException)
            {
            }
        }

        private static Label Lbl(string text, int x, int y)
        {
            var l = new Label();
            l.SetBounds(x, y, 70, 20);
            l.Text = text + ":";
            // The label's own accessible name is the field's name WITHOUT the colon, so a
            // search for "Name" is not ambiguous between the label and the field it labels in a
            // way that depends on punctuation.
            l.AccessibleName = text + " label";
            return l;
        }

        private void OnSave(object sender, EventArgs e)
        {
            savePresses++;
            if (stubborn)
            {
                // ACCEPTED AND IGNORED. The press is counted — the oracle must be able to say
                // "Marco did press it" — and nothing else changes. Marco's own verification has
                // to be what fails here.
                Write();
                return;
            }
            status.Text = "Status: Saved";
            Write();
        }

        private void OnCancel(object sender, EventArgs e)
        {
            cancelPresses++;
            name.Text = "";
            notes.Text = "";
            remember.Checked = false;
            items.ClearSelected();
            status.Text = "Status: Cancelled";
            Write();
        }

        private void OnDetails(object sender, EventArgs e)
        {
            detailsPresses++;
            secondText.Text = items.SelectedItem == null
                ? "Nothing selected"
                : "Selected: " + items.SelectedItem;
            secondView.Visible = true;
            status.Text = "Status: Details";
            Write();
        }

        /// <summary>Writes the oracle. Marco must never read this.</summary>
        ///
        /// Written on EVERY change rather than on a request, so the harness never has to ask the
        /// fixture anything — asking would mean a channel between the harness and the application
        /// under test, and a channel is a thing that can be used by accident.
        ///
        /// Atomically, via a temp file and a replace, because the harness reads it while the
        /// fixture is running and a half-written file reads as a failure that did not happen.
        private void Write()
        {
            var b = new StringBuilder();
            b.Append("{");
            // THE WINDOW'S OWN HANDLE, so the harness can scope a perception read to THIS
            // fixture rather than to "whatever is called fixture".
            //
            // Measured: a fixture left running by an earlier manual session made
            // `--window fixture` ambiguous, Marco refused to read either window, and a case
            // in which Marco had actually pressed the button reported failure. The harness was
            // asking about an application when it meant a window.
            //
            // It is NOT a hint about what is on screen: it narrows WHERE Marco looks, exactly
            // as acting on the foreground does, and says nothing about what it will find.
            b.Append("\"hwnd\":")
             .Append(Handle.ToInt64().ToString(CultureInfo.InvariantCulture)).Append(",");
            Field(b, "title", Text); b.Append(",");
            Field(b, "name", name.Text); b.Append(",");
            Field(b, "notes", notes.Text); b.Append(",");
            b.Append("\"remember\":").Append(remember.Checked ? "true" : "false").Append(",");
            Field(b, "selected", items.SelectedItem == null ? "" : items.SelectedItem.ToString());
            b.Append(",");
            Field(b, "status", status.Text); b.Append(",");
            Field(b, "detail", secondText.Text); b.Append(",");
            // WHETHER THE TREE IS OPEN, so an expand has an independent witness. Read from the
            // control itself rather than from its accessible object -- which is the distinction the
            // selection measurement made worth writing down: a provider can report a state the
            // application does not have, at every level it exposes.
                        b.Append("\"documents_expanded\":")
             .Append(documentsExpanded ? "true" : "false").Append(",");
            b.Append("\"expand_events\":")
             .Append(expandEvents.ToString(CultureInfo.InvariantCulture)).Append(",");
            b.Append("\"collapse_events\":")
             .Append(collapseEvents.ToString(CultureInfo.InvariantCulture)).Append(",");
            b.Append("\"second_view\":").Append(secondView.Visible ? "true" : "false").Append(",");
            b.Append("\"save_presses\":").Append(savePresses.ToString(CultureInfo.InvariantCulture));
            b.Append(",");
            b.Append("\"cancel_presses\":")
             .Append(cancelPresses.ToString(CultureInfo.InvariantCulture)).Append(",");
            b.Append("\"details_presses\":")
             .Append(detailsPresses.ToString(CultureInfo.InvariantCulture)).Append(",");
            b.Append("\"stubborn\":").Append(stubborn ? "true" : "false");
            b.Append("}");

            try
            {
                var tmp = statePath + ".tmp";
                File.WriteAllText(tmp, b.ToString(), new UTF8Encoding(false));
                if (File.Exists(statePath)) { File.Delete(statePath); }
                File.Move(tmp, statePath);
            }
            catch (IOException)
            {
                // A write the harness missed is a read it retries. Losing the window over a
                // locked file would turn a flaky disk into a failed acceptance.
            }
        }

        private static void Field(StringBuilder b, string key, string value)
        {
            b.Append("\"").Append(key).Append("\":\"");
            foreach (char c in value)
            {
                switch (c)
                {
                    case '"': b.Append("\\\""); break;
                    case '\\': b.Append("\\\\"); break;
                    case '\n': b.Append("\\n"); break;
                    case '\r': b.Append("\\r"); break;
                    case '\t': b.Append("\\t"); break;
                    default:
                        if (c < 0x20) { b.Append("\\u").Append(((int)c).ToString("x4")); }
                        else { b.Append(c); }
                        break;
                }
            }
            b.Append("\"");
        }
    }
}
