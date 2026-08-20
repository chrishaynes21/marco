using System;
using System.Collections.Generic;
using System.Text;

namespace MarcoUia
{
    /// <summary>Setting and reading a control's value.
    ///
    /// The strongest way to change text, and the reason is not speed. Typing sends
    /// characters through the keyboard layout, the IME, and whatever the application
    /// does with each keystroke — autocomplete, validation, a dropdown that steals
    /// focus on the third letter. SetValue sets the value: no intermediate state for
    /// anything to react to, and no layout to get wrong.
    ///
    /// Not every control supports it. Read-only fields refuse and many custom controls
    /// never implement the pattern, so "unsupported" is reported distinctly from a real
    /// error — the caller falls back deliberately on the first and stops on the second.
    /// Collapsing them would make every read-only field look like a broken bridge.</summary>
    internal static partial class Program
    {
        static string ValueReply(Dictionary<string, object> input, bool write)
        {
            string id = JsonReader.Str(input, "Element");
            if (id.Length == 0) return Failed("this action needs an Element id");

            IntPtr hwnd = IntPtr.Zero;
            string w = JsonReader.Str(input, "Window");
            if (w.Length > 0)
            {
                long h;
                if (!w.StartsWith("hwnd:", StringComparison.OrdinalIgnoreCase) ||
                    !long.TryParse(w.Substring(5), out h) || h == 0)
                {
                    return Failed("Window must look like hwnd:<handle>");
                }
                hwnd = new IntPtr(h);
            }

            int budget = JsonReader.Int(input, "MaxNodes", 1500);
            string value = "";
            string problem = write
                ? Focuser.SetValue(hwnd, id, JsonReader.Str(input, "Value"), budget)
                : Focuser.GetValue(hwnd, id, budget, out value);
            if (problem.Length > 0) return Failed(problem);

            // After a write, read the value BACK. The reply then carries what the control
            // actually holds rather than what it was asked to hold — which is the whole
            // basis of verification, and is not always the same thing: a field with a
            // length cap or an input mask accepts a SetValue and keeps something else.
            if (write) Focuser.GetValue(hwnd, id, budget, out value);

            var sb = new StringBuilder();
            var jw = new JsonWriter(sb);
            jw.ObjStart();
            jw.Field("status", "ok"); jw.Sep();
            jw.Key("data");
            jw.ObjStart();
            jw.Field("Value", value);
            jw.ObjEnd();
            jw.ObjEnd();
            return sb.ToString();
        }
    }
}
