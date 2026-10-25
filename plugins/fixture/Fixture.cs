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
// # The drain barrier
//
//	--state <p> also answers a question: the harness writes <p>.drain and the window
//	writes <p>.drained once its message loop has gone empty.
//
// An input log is written when the window PROCESSES a message, not when the system accepts it.
// So a harness that read the log the moment the thing sending input stopped would be reading a
// window that may still have presses queued, and "nothing arrived" would be "nothing has arrived
// YET" — which is the false green a refusal scenario is entirely made of. The barrier is the
// window saying, from inside its own loop, that there was nothing left to process.
//
//	--selftest   stage a press in this window's own queue and stop processing until
//	             released, so a harness can prove its barrier against a window that
//	             really is behind. Never used by an acceptance.
//
// Build:  powershell -File plugins/fixture/build.ps1
// Run:    plugins/fixture/fixture.exe --state <path> [--stubborn] [--view second]

using System;
using System.Diagnostics;
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
            string events = null;
            bool stubborn = false;
            bool selftest = false;
            for (int i = 0; i < args.Length; i++)
            {
                switch (args[i])
                {
                    case "--state":
                        if (i + 1 < args.Length) { state = args[++i]; }
                        break;
                    case "--events":
                        if (i + 1 < args.Length) { events = args[++i]; }
                        break;
                    case "--stubborn":
                        stubborn = true;
                        break;
                    case "--selftest":
                        selftest = true;
                        break;
                    default:
                        Console.Error.WriteLine("fixture: unknown argument " + args[i]);
                        return 2;
                }
            }
            if (string.IsNullOrEmpty(state))
            {
                Console.Error.WriteLine("usage: fixture.exe --state <path> " +
                    "[--events <path>] [--stubborn] [--selftest]");
                return 2;
            }

            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);
            Application.Run(new FixtureForm(state, stubborn, events, selftest));
            return 0;
        }
    }

    /// <summary>The one window. Deterministic in title, layout, control names and position.</summary>
    internal sealed class FixtureForm : Form
    {
        private readonly string statePath;
        private readonly bool stubborn;

        // ── THE WITNESS: WHAT THIS WINDOW RECEIVED, AS OPPOSED TO WHAT IT NOW HOLDS ──
        //
        // # Why the state file was not enough
        //
        // The oracle beside this one is a SNAPSHOT: what the fields hold, what is checked, how
        // many times each button has been pressed. It answers "did Save get pressed" and cannot
        // answer "where did the press land", "in what order", "was anything else pressed", or
        // "what did the window look like at that instant". A press that misses Save by four
        // pixels and lands on the panel behind it is, to a snapshot, indistinguishable from no
        // press at all — and a coordinate replay is exactly the thing that fails that way.
        //
        // So this is an append-only log of INPUT RECEIVED, written from a message filter before
        // the controls see the message, with the window's own geometry stamped on every record.
        // That last part is what makes a moved or resized window checkable without
        // cross-referencing anything: the press says where it arrived AND what the window was at
        // the time it arrived.
        //
        // THE SAME RULE AS THE STATE FILE APPLIES: Marco must never read it. A log Marco could
        // read would turn "Marco pressed Save" into "Marco can read a file".
        private readonly string eventsPath;
        private readonly object eventsLock = new object();
        private StreamWriter events;
        private int seq;
        private readonly Stopwatch clock = Stopwatch.StartNew();
        private int dpi = 96;
        private Rectangle lastWindow = Rectangle.Empty;

        // ── THE DRAIN BARRIER: "IS THERE ANYTHING YOU HAVE NOT PROCESSED YET?" ──
        //
        // # The false green this exists to close
        //
        // Input is logged when this window's message loop DISPATCHES it, which is not when the
        // system accepted it. So between a press being sent and this window writing it down
        // there is an interval in which the press exists and the log does not mention it. A
        // harness that read the log as soon as the thing sending input finished would be
        // reading that interval and calling it absence — and every scenario whose claim is
        // "nothing arrived" is exactly one that reads the log after the sender stops.
        //
        // An interval would not fix it either: waiting two seconds and finding silence
        // establishes that nothing has arrived YET.
        //
        // # Why it is an idle and not an acknowledged message
        //
        // Because a POSTED message is retrieved AHEAD of queued input — `GetMessage` serves
        // sent, then posted, then input — so a sentinel posted to this window would come back
        // with the presses still waiting behind it, which is worse than no barrier at all: it
        // would look like proof. What this counts instead is the loop running OUT of
        // everything. An idle after the request means the queue held no input at that moment,
        // so whatever was in it has been dispatched and written down.
        //
        // # And what it carries
        //
        // A token and nothing else. The answer says "the thing you asked about has happened"
        // and says nothing about what this window received, which stays where it belongs: in
        // the two oracles the harness reads for itself. A channel that reported facts would be
        // a channel the harness could be tempted to believe instead of looking.
        private readonly string drainRequest;
        private readonly string drainAnswer;
        private int idles;
        private string drainToken;
        private int drainAt;

        // ── AND THE MODE THAT PROVES THE BARRIER IS ONE ──
        //
        // A barrier nobody has seen hold is an assumption with a function name. This mode makes
        // this window genuinely behind: it puts a press in its own queue and then stops
        // processing anything until the harness releases it, so the harness can read the log
        // while a press is pending (it must not be there), cross the barrier, and read it again
        // (it must). Without the barrier, the first read is the false green in person.
        //
        // THE PRESS IS POSTED, BY THIS WINDOW, INTO ITS OWN QUEUE. It is not input: nothing is
        // injected into the desktop, no pointer moves, and nothing Marco can reach is told that
        // a person did anything. What is under test here is the HARNESS's synchronisation, not
        // Marco — so the thing that has to be pending is a message, and the cheapest honest way
        // to have one pending is for this window to put it there.
        //
        // Not done with a real run instead, which was the first design: a window that stops
        // pumping for more than about five seconds gets GHOSTED by Windows — a stand-in window
        // with its own handle takes its place in the foreground — and the run would then be
        // refused by Marco's own window guard for a reason that has nothing to do with the
        // barrier.
        private readonly bool selftest;
        private readonly string selftestRequest;
        private readonly string selftestBlocked;
        private readonly string selftestRelease;

        private readonly TextBox name = new TextBox();
        private readonly TextBox notes = new TextBox();
        private readonly CheckBox remember = new CheckBox();
        private readonly ListBox items = new ListBox();

        // A SCROLLABLE REGION, so a view movement has somewhere to happen and an independent
        // witness when it does.
        //
        // A long ListBox rather than a text box: TopIndex is the managed control's own answer to
        // "where is the view", which is exactly the kind of oracle this fixture exists to provide
        // -- read from the control and not from its accessible object. The selection measurement
        // is why that distinction is written down here: a provider can report a state the
        // application does not have.
        private readonly ListBox lines = new ListBox();

        // lastTopIndex is what the state file last reported, so the poll below writes on CHANGE
        // rather than on every tick.
        private int lastTopIndex = -1;
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

        internal FixtureForm(string statePath, bool stubborn, string eventsPath, bool selftest)
        {
            this.statePath = statePath;
            this.stubborn = stubborn;
            this.eventsPath = eventsPath;
            this.selftest = selftest;
            drainRequest = statePath + ".drain";
            drainAnswer = statePath + ".drained";
            selftestRequest = statePath + ".selftest";
            selftestBlocked = statePath + ".blocked";
            selftestRelease = statePath + ".release";

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

            // 200 rows in a 76px box: a view that must scroll, and far enough that a page step
            // and a jump to an end are plainly different outcomes.
            lines.SetBounds(300, 120, 140, 76);
            lines.AccessibleName = "Lines";
            lines.TabIndex = 9;
            for (int i = 1; i <= 200; i++)
                lines.Items.Add("Row " + i.ToString(CultureInfo.InvariantCulture));
            lines.SelectedIndexChanged += delegate { Write(); };

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
            ClientSize = new Size(620, 440);

            Controls.AddRange(new Control[]
            {
                nameLabel, name, notesLabel, notes, remember,
                itemsLabel, items, lines, save, cancel, details, deleteBtn,
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

            // A VIEW MOVEMENT HAS NO EVENT, so the oracle is polled for this one fact.
            //
            // WinForms raises nothing when a ListBox scrolls -- no selection changes, no
            // property notifies -- so a state file written only on the events the other
            // controls raise would report the view where it was before the scroll. That is a
            // STALE oracle, which is worse than a missing one: it disagrees with Marco while
            // looking like independent ground truth. Measured exactly that way before this
            // existed, with UIA reporting 0% -> 100% while the file still said top_index 0.
            //
            // Compared rather than written every tick, so the file changes when the view does
            // and a reader can tell the difference.
            var viewWatch = new Timer();
            viewWatch.Interval = 100;
            viewWatch.Tick += delegate
            {
                if (lines.TopIndex != lastTopIndex) { lastTopIndex = lines.TopIndex; Write(); }
            };
            viewWatch.Start();

            // ── AND THE WITNESS, LAST, so nothing above it is logged as a press ──
            //
            // The filter is installed AFTER the window is built because building it sets values
            // and raises events; an input log that opened with the construction of the form would
            // begin with records no hand produced.
            OpenEvents();
            Application.AddMessageFilter(new Witness(this));

            // ── AND THE COUNT OF TIMES THIS LOOP RAN OUT OF WORK ──
            //
            // Idle is raised when the pump finds nothing left to dispatch — no sent message, no
            // posted message, and no input. That is the only observation in this process that
            // means "the queue is empty", which is the one thing the barrier has to establish.
            Application.Idle += delegate { idles++; };
            var asked = new Timer();
            asked.Interval = 50;
            asked.Tick += delegate { Drain(); Stage(); };
            asked.Start();

            Shown += delegate { NoteWindow("shown"); };
            Move += delegate { NoteWindow("moved"); };
            Resize += delegate { NoteWindow("resized"); };
            FormClosed += delegate { CloseEvents(); };
        }

        // ── THE INPUT WITNESS ─────────────────────────────────────────────────

        /// <summary>Logs every mouse and key message this window receives, and changes none.</summary>
        ///
        /// A message filter rather than per-control handlers, for three reasons. It sees the
        /// message BEFORE the control does, so what is logged is what arrived rather than what a
        /// handler concluded. It covers every control and the form's own background with one
        /// implementation, so a press that lands between two buttons is recorded instead of
        /// vanishing — which is the whole difference between "clicked nothing" and "clicked the
        /// wrong thing". And it covers keys going to whichever control has focus, which
        /// KeyPreview on the form would see only for its own children.
        ///
        /// PreFilterMessage returns FALSE, always. A witness that swallowed a message would be an
        /// application behaving differently because it was being watched.
        private sealed class Witness : IMessageFilter
        {
            private const int WmMouseMove = 0x0200;
            private const int WmLButtonDown = 0x0201, WmLButtonUp = 0x0202;
            private const int WmRButtonDown = 0x0204, WmRButtonUp = 0x0205;
            private const int WmMButtonDown = 0x0207, WmMButtonUp = 0x0208;
            private const int WmKeyDown = 0x0100, WmKeyUp = 0x0101;
            private const int WmSysKeyDown = 0x0104, WmSysKeyUp = 0x0105;

            private readonly FixtureForm form;

            internal Witness(FixtureForm form) { this.form = form; }

            public bool PreFilterMessage(ref Message m)
            {
                switch (m.Msg)
                {
                    case WmLButtonDown: form.NoteClick(m, "left", true); break;
                    case WmLButtonUp: form.NoteClick(m, "left", false); break;
                    case WmRButtonDown: form.NoteClick(m, "right", true); break;
                    case WmRButtonUp: form.NoteClick(m, "right", false); break;
                    case WmMButtonDown: form.NoteClick(m, "middle", true); break;
                    case WmMButtonUp: form.NoteClick(m, "middle", false); break;
                    case WmKeyDown: form.NoteKey(m, true, false); break;
                    case WmSysKeyDown: form.NoteKey(m, true, true); break;
                    case WmKeyUp: form.NoteKey(m, false, false); break;
                    case WmSysKeyUp: form.NoteKey(m, false, true); break;
                    case WmMouseMove: break; // Deliberately not logged: see NoteClick.
                }
                return false;
            }
        }

        private void OpenEvents()
        {
            if (string.IsNullOrEmpty(eventsPath)) { return; }
            using (Graphics g = CreateGraphics()) { dpi = (int)Math.Round(g.DpiX); }
            try
            {
                events = new StreamWriter(eventsPath, false, new UTF8Encoding(false));
                events.AutoFlush = true;
            }
            catch (IOException)
            {
                events = null;
                return;
            }
            // THE FIRST RECORD SAYS WHAT THIS WINDOW IS, so a log can be read on its own: which
            // process and handle produced it, and the geometry every later record is relative to.
            var b = new StringBuilder();
            b.Append("{\"seq\":0,\"ms\":0,\"kind\":\"start\",");
            Field(b, "title", Text); b.Append(",");
            b.Append("\"pid\":").Append(
                System.Diagnostics.Process.GetCurrentProcess().Id
                    .ToString(CultureInfo.InvariantCulture)).Append(",");
            b.Append("\"hwnd\":").Append(
                Handle.ToInt64().ToString(CultureInfo.InvariantCulture)).Append(",");
            b.Append("\"stubborn\":").Append(stubborn ? "true" : "false").Append(",");
            Geometry(b); b.Append(",");
            ControlRects(b);
            b.Append("}");
            WriteEvent(b.ToString());
        }

        private void CloseEvents()
        {
            lock (eventsLock)
            {
                if (events == null) { return; }
                try { events.Flush(); events.Dispose(); }
                catch (IOException) { }
                events = null;
            }
        }

        private void WriteEvent(string line)
        {
            lock (eventsLock)
            {
                if (events == null) { return; }
                try { events.WriteLine(line); }
                catch (IOException)
                {
                    // A line the harness did not get is a missing record, and a missing record
                    // fails the comparison it belongs to. Losing the window over a locked file
                    // would turn that into a crash instead of a finding.
                }
            }
        }

        /// <summary>Logs one mouse button edge: where it landed, on what, and in what window.</summary>
        ///
        /// MOVES ARE NOT LOGGED. A replay sends a move before every click and a person moves the
        /// pointer continuously, so logging them would bury the presses in thousands of records
        /// and make the log useless as evidence. What a press needs is where it ARRIVED, and that
        /// is on the press.
        ///
        /// Three coordinate spaces, because they answer three different questions and a harness
        /// that conflated them would be unable to say which one disagreed:
        ///
        ///   on_control — inside the control that got it. Was the press ON the button, and where?
        ///   client     — inside this window. Comparable with the control rectangles published
        ///                beside it, so a press that hit nothing can still be placed.
        ///   screen     — where the operating system put it, in this process's own idea of the
        ///                screen. The only space a replayed coordinate can be compared against.
        private void NoteClick(Message m, string button, bool down)
        {
            int lp = (int)(long)m.LParam;
            var onControl = new Point((short)(lp & 0xffff), (short)((lp >> 16) & 0xffff));
            Control c = Control.FromHandle(m.HWnd);
            Point screen = c != null ? c.PointToScreen(onControl) : onControl;
            Point client = PointToClient(screen);
            var b = new StringBuilder();
            Head(b, "mouse");
            Field(b, "what", button + (down ? "_down" : "_up")); b.Append(",");
            Field(b, "control", NameOf(c)); b.Append(",");
            b.Append("\"on_control_x\":").Append(N(onControl.X)).Append(",");
            b.Append("\"on_control_y\":").Append(N(onControl.Y)).Append(",");
            b.Append("\"client_x\":").Append(N(client.X)).Append(",");
            b.Append("\"client_y\":").Append(N(client.Y)).Append(",");
            b.Append("\"screen_x\":").Append(N(screen.X)).Append(",");
            b.Append("\"screen_y\":").Append(N(screen.Y)).Append(",");
            // AND WHAT THE WINDOW WAS AT THE INSTANT IT ARRIVED. Stamped on the record rather
            // than left to be looked up: a harness that read the geometry afterwards would be
            // describing the window as it is now and attributing it to a press from before.
            Geometry(b);
            b.Append("}");
            WriteEvent(b.ToString());
        }

        private void NoteKey(Message m, bool down, bool system)
        {
            var key = (Keys)(int)(long)m.WParam;
            var b = new StringBuilder();
            Head(b, "key");
            Field(b, "what", down ? "down" : "up"); b.Append(",");
            Field(b, "key", key.ToString()); b.Append(",");
            b.Append("\"vk\":").Append(N((int)key)).Append(",");
            b.Append("\"system\":").Append(system ? "true" : "false").Append(",");
            Field(b, "control", NameOf(Control.FromHandle(m.HWnd))); b.Append(",");
            Geometry(b);
            b.Append("}");
            WriteEvent(b.ToString());
        }

        private void NoteWindow(string what)
        {
            var now = new Rectangle(Location, Size);
            if (what != "shown" && now == lastWindow) { return; }
            lastWindow = now;
            var b = new StringBuilder();
            Head(b, "window");
            Field(b, "what", what); b.Append(",");
            Geometry(b); b.Append(",");
            ControlRects(b);
            b.Append("}");
            WriteEvent(b.ToString());
            // AND THE SNAPSHOT TOO, so a reader of either file sees the same window.
            Write();
        }

        private void Head(StringBuilder b, string kind)
        {
            b.Append("{\"seq\":").Append(N(++seq)).Append(",");
            b.Append("\"ms\":").Append(
                clock.ElapsedMilliseconds.ToString(CultureInfo.InvariantCulture)).Append(",");
            Field(b, "kind", kind); b.Append(",");
        }

        /// <summary>This window's geometry and scale, as the window itself reports them.</summary>
        ///
        /// From the form rather than from any accessibility reading of it, which is the same
        /// distinction the view and tree measurements already make: a provider can report a state
        /// the application does not have. `client` is given with its SCREEN origin, because the
        /// one arithmetic a harness actually needs is control rectangle plus client origin.
        private void Geometry(StringBuilder b)
        {
            Point origin = PointToScreen(Point.Empty);
            b.Append("\"window\":{\"x\":").Append(N(Location.X))
             .Append(",\"y\":").Append(N(Location.Y))
             .Append(",\"w\":").Append(N(Size.Width))
             .Append(",\"h\":").Append(N(Size.Height)).Append("},");
            b.Append("\"client\":{\"x\":").Append(N(origin.X))
             .Append(",\"y\":").Append(N(origin.Y))
             .Append(",\"w\":").Append(N(ClientSize.Width))
             .Append(",\"h\":").Append(N(ClientSize.Height)).Append("},");
            b.Append("\"dpi\":").Append(N(dpi));
        }

        /// <summary>Every control that has a name, and where it is in this window.</summary>
        ///
        /// Walked rather than listed, so a control added to this fixture is published without
        /// anybody remembering to add it here — and keyed by AccessibleName, which is the word a
        /// person uses for it and the word Marco reads.
        private void ControlRects(StringBuilder b)
        {
            b.Append("\"controls\":{");
            bool first = true;
            AddControls(b, this, ref first);
            b.Append("}");
        }

        private void AddControls(StringBuilder b, Control parent, ref bool first)
        {
            foreach (Control c in parent.Controls)
            {
                string called = c.AccessibleName;
                if (!string.IsNullOrEmpty(called))
                {
                    Rectangle r = RectangleToClient(c.RectangleToScreen(c.ClientRectangle));
                    if (!first) { b.Append(","); }
                    first = false;
                    b.Append("\"");
                    Escape(b, called);
                    b.Append("\":{\"x\":").Append(N(r.X))
                     .Append(",\"y\":").Append(N(r.Y))
                     .Append(",\"w\":").Append(N(r.Width))
                     .Append(",\"h\":").Append(N(r.Height))
                     .Append(",\"visible\":").Append(c.Visible ? "true" : "false")
                     .Append(",\"enabled\":").Append(c.Enabled ? "true" : "false")
                     .Append("}");
                }
                AddControls(b, c, ref first);
            }
        }

        private static string NameOf(Control c)
        {
            if (c == null) { return ""; }
            if (!string.IsNullOrEmpty(c.AccessibleName)) { return c.AccessibleName; }
            return c.GetType().Name;
        }

        private static string N(int v)
        {
            return v.ToString(CultureInfo.InvariantCulture);
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

        /// <summary>Answers the harness's drain request, once this loop has run out of work.</summary>
        ///
        /// Two ticks at least, and that is the whole mechanism. The first notices the request and
        /// writes down the idle count it must get PAST; a later one answers, once an idle has
        /// happened after that. An idle strictly after the request means the queue was empty
        /// after the question was asked, so anything that had been queued when it was asked has
        /// been dispatched — and dispatched is when the witness writes it down.
        ///
        /// What this does NOT establish, said here because the harness quotes it: input the
        /// system has accepted but not yet placed in this queue. That handoff is done by a system
        /// thread and is not observable from inside this process at all.
        private void Drain()
        {
            if (drainToken == null)
            {
                string token;
                try
                {
                    if (!File.Exists(drainRequest)) { return; }
                    token = File.ReadAllText(drainRequest).Trim();
                    File.Delete(drainRequest);
                }
                catch (IOException)
                {
                    return;
                }
                if (token.Length == 0) { return; }
                drainToken = token;
                // THE IDLE THIS ANSWER WAITS FOR IS A LATER ONE. Read inside a tick, so the
                // pump is demonstrably not idle now, and the next increment can only come
                // from the loop emptying after this moment.
                drainAt = idles + 1;
                return;
            }
            if (idles < drainAt) { return; }
            try
            {
                File.WriteAllText(drainAnswer, drainToken, new UTF8Encoding(false));
            }
            catch (IOException)
            {
                return;
            }
            drainToken = null;
        }

        [DllImport("user32.dll")]
        private static extern bool PostMessage(IntPtr h, int msg, IntPtr w, IntPtr l);

        /// <summary>Makes this window genuinely behind, so a harness can prove its barrier.</summary>
        ///
        /// A press goes into this window's own queue and then the loop stops running until the
        /// harness releases it. While it is stopped the press exists and the log does not mention
        /// it, which is the state every absence assertion is read in — so a harness can look
        /// there (and must see nothing), cross the barrier, and look again (and must see it).
        ///
        /// Blocking is the point. Nothing is pumped here: a `DoEvents` would dispatch the very
        /// message this is holding, and a window that answered while it was behind would be the
        /// bug this mode exists to detect.
        private void Stage()
        {
            if (!selftest) { return; }
            try
            {
                if (!File.Exists(selftestRequest)) { return; }
                File.Delete(selftestRequest);
            }
            catch (IOException)
            {
                return;
            }

            const int WmLButtonDown = 0x0201, WmLButtonUp = 0x0202;
            const int MkLButton = 0x0001;
            int at = (save.ClientSize.Height / 2) << 16 | (save.ClientSize.Width / 2);
            PostMessage(save.Handle, WmLButtonDown, (IntPtr)MkLButton, (IntPtr)at);
            PostMessage(save.Handle, WmLButtonUp, IntPtr.Zero, (IntPtr)at);

            // AND THE HARNESS IS TOLD THAT IT IS NOW SAFE TO LOOK.
            //
            // Written after the press is queued and before anything is dispatched, so the file
            // appearing means exactly one thing: there is a press in this window that this
            // window has not seen. A harness that had to guess that instant would be timing.
            try
            {
                File.WriteAllText(selftestBlocked, "a press is queued and unprocessed",
                    new UTF8Encoding(false));
            }
            catch (IOException)
            {
            }
            var waited = Stopwatch.StartNew();
            while (!File.Exists(selftestRelease) && waited.ElapsedMilliseconds < 60000)
            {
                System.Threading.Thread.Sleep(25);
            }
            try
            {
                if (File.Exists(selftestRelease)) { File.Delete(selftestRelease); }
                File.Delete(selftestBlocked);
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
            // WHERE THIS WINDOW IS, HOW BIG IT IS, AND AT WHAT SCALE, with every named control's
            // rectangle inside it.
            //
            // Published because a harness that replays a coordinate has to be able to say what
            // that coordinate MEANT. Without this it can assert that Save was pressed and cannot
            // assert where the press landed, which is the one thing a coordinate replay can get
            // wrong while still looking right. Read from the form rather than from any
            // accessibility reading of it, for the same reason the view and tree measurements are:
            // a provider can report a state the application does not have.
            Geometry(b); b.Append(",");
            ControlRects(b); b.Append(",");
            Field(b, "name", name.Text); b.Append(",");
            Field(b, "notes", notes.Text); b.Append(",");
            b.Append("\"remember\":").Append(remember.Checked ? "true" : "false").Append(",");
            Field(b, "selected", items.SelectedItem == null ? "" : items.SelectedItem.ToString());
            b.Append(",");
            Field(b, "status", status.Text); b.Append(",");
            // WHERE THE SCROLLABLE VIEW IS, from the control's own TopIndex rather than from any
            // accessibility reading of it -- the independent channel for a view movement.
            b.Append("\"lines_top_index\":")
             .Append(lines.TopIndex.ToString(CultureInfo.InvariantCulture)).Append(",");
            b.Append("\"lines_count\":")
             .Append(lines.Items.Count.ToString(CultureInfo.InvariantCulture)).Append(",");
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
            Escape(b, value);
            b.Append("\"");
        }

        /// <summary>Appends a JSON-safe string body. Used for VALUES and for control names,
        /// which are keys made of data rather than of literals.</summary>
        private static void Escape(StringBuilder b, string value)
        {
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
        }
    }
}
