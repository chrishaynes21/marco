// Minimal JSON reader/writer.
//
// The bridge protocol is newline-delimited JSON, and this plugin needs to parse a
// tiny request object and emit a possibly large response. .NET Framework's own
// options (JavaScriptSerializer, DataContractJsonSerializer) both pull in extra
// assembly references, and the whole point of this plugin is that it compiles with
// the csc.exe every Windows machine already has and needs nothing installed. ~200
// lines of hand-rolled JSON is a cheaper dependency than a build prerequisite.
//
// The writer is the part that matters: it is used for every element of every
// snapshot, so it appends to a shared StringBuilder rather than building strings.

using System;
using System.Collections.Generic;
using System.Globalization;
using System.Text;

namespace MarcoUia
{
    /// <summary>Reads a JSON value into object/Dictionary/List/string/double/bool/null.</summary>
    internal static class JsonReader
    {
        public static object Parse(string s)
        {
            int i = 0;
            object v = ParseValue(s, ref i);
            return v;
        }

        static void SkipWhite(string s, ref int i)
        {
            while (i < s.Length && (s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n')) i++;
        }

        static object ParseValue(string s, ref int i)
        {
            SkipWhite(s, ref i);
            if (i >= s.Length) return null;
            switch (s[i])
            {
                case '{': return ParseObject(s, ref i);
                case '[': return ParseArray(s, ref i);
                case '"': return ParseString(s, ref i);
                case 't': i += 4; return true;
                case 'f': i += 5; return false;
                case 'n': i += 4; return null;
                default: return ParseNumber(s, ref i);
            }
        }

        static Dictionary<string, object> ParseObject(string s, ref int i)
        {
            var d = new Dictionary<string, object>(StringComparer.OrdinalIgnoreCase);
            i++; // {
            SkipWhite(s, ref i);
            if (i < s.Length && s[i] == '}') { i++; return d; }
            while (i < s.Length)
            {
                SkipWhite(s, ref i);
                if (i >= s.Length || s[i] != '"') break;
                string key = ParseString(s, ref i);
                SkipWhite(s, ref i);
                if (i < s.Length && s[i] == ':') i++;
                d[key] = ParseValue(s, ref i);
                SkipWhite(s, ref i);
                if (i < s.Length && s[i] == ',') { i++; continue; }
                if (i < s.Length && s[i] == '}') { i++; break; }
                break;
            }
            return d;
        }

        static List<object> ParseArray(string s, ref int i)
        {
            var a = new List<object>();
            i++; // [
            SkipWhite(s, ref i);
            if (i < s.Length && s[i] == ']') { i++; return a; }
            while (i < s.Length)
            {
                a.Add(ParseValue(s, ref i));
                SkipWhite(s, ref i);
                if (i < s.Length && s[i] == ',') { i++; continue; }
                if (i < s.Length && s[i] == ']') { i++; break; }
                break;
            }
            return a;
        }

        static string ParseString(string s, ref int i)
        {
            var sb = new StringBuilder();
            i++; // opening quote
            while (i < s.Length && s[i] != '"')
            {
                if (s[i] == '\\' && i + 1 < s.Length)
                {
                    i++;
                    switch (s[i])
                    {
                        case 'n': sb.Append('\n'); break;
                        case 't': sb.Append('\t'); break;
                        case 'r': sb.Append('\r'); break;
                        case 'b': sb.Append('\b'); break;
                        case 'f': sb.Append('\f'); break;
                        case 'u':
                            if (i + 4 < s.Length)
                            {
                                sb.Append((char)Convert.ToInt32(s.Substring(i + 1, 4), 16));
                                i += 4;
                            }
                            break;
                        default: sb.Append(s[i]); break;
                    }
                    i++;
                    continue;
                }
                sb.Append(s[i]);
                i++;
            }
            i++; // closing quote
            return sb.ToString();
        }

        static object ParseNumber(string s, ref int i)
        {
            int start = i;
            while (i < s.Length && "-+.eE0123456789".IndexOf(s[i]) >= 0) i++;
            double d;
            if (double.TryParse(s.Substring(start, i - start), NumberStyles.Float,
                                CultureInfo.InvariantCulture, out d))
                return d;
            return null;
        }

        /// <summary>Reads a string field, or "" when absent.</summary>
        public static string Str(Dictionary<string, object> d, string key)
        {
            object v;
            if (d != null && d.TryGetValue(key, out v) && v != null) return v.ToString();
            return "";
        }

        /// <summary>Reads a numeric field, or fallback when absent or not a number.</summary>
        public static int Int(Dictionary<string, object> d, string key, int fallback)
        {
            object v;
            if (d != null && d.TryGetValue(key, out v) && v is double) return (int)(double)v;
            return fallback;
        }

        /// <summary>Reads an object field, or null.</summary>
        public static Dictionary<string, object> Obj(Dictionary<string, object> d, string key)
        {
            object v;
            if (d != null && d.TryGetValue(key, out v)) return v as Dictionary<string, object>;
            return null;
        }
    }

    /// <summary>Appends JSON to a StringBuilder. Comma placement is the caller's job
    /// via Sep(), which keeps the writer allocation-free on the hot path.</summary>
    internal sealed class JsonWriter
    {
        readonly StringBuilder sb;
        public JsonWriter(StringBuilder target) { sb = target; }

        public void Raw(string s) { sb.Append(s); }
        public void Sep() { sb.Append(','); }
        public void ObjStart() { sb.Append('{'); }
        public void ObjEnd() { sb.Append('}'); }
        public void ArrStart() { sb.Append('['); }
        public void ArrEnd() { sb.Append(']'); }

        public void Key(string name)
        {
            String(name);
            sb.Append(':');
        }

        public void String(string s)
        {
            sb.Append('"');
            if (s != null)
            {
                foreach (char c in s)
                {
                    switch (c)
                    {
                        case '"': sb.Append("\\\""); break;
                        case '\\': sb.Append("\\\\"); break;
                        case '\n': sb.Append("\\n"); break;
                        case '\r': sb.Append("\\r"); break;
                        case '\t': sb.Append("\\t"); break;
                        case '\b': sb.Append("\\b"); break;
                        case '\f': sb.Append("\\f"); break;
                        default:
                            // Control characters (and the lone surrogates a UI label can
                            // contain) must be escaped or the Go side fails to decode.
                            if (c < 0x20 || (c >= 0xD800 && c <= 0xDFFF))
                                sb.Append("\\u").Append(((int)c).ToString("x4"));
                            else
                                sb.Append(c);
                            break;
                    }
                }
            }
            sb.Append('"');
        }

        public void Number(int n) { sb.Append(n.ToString(CultureInfo.InvariantCulture)); }
        public void Number(double n) { sb.Append(n.ToString("0.####", CultureInfo.InvariantCulture)); }
        public void Bool(bool b) { sb.Append(b ? "true" : "false"); }
        public void Null() { sb.Append("null"); }

        public void Field(string name, string value) { Key(name); String(value); }
        public void Field(string name, int value) { Key(name); Number(value); }
        public void Field(string name, bool value) { Key(name); Bool(value); }

        // Invariant culture, explicitly. A machine with a comma decimal separator would
        // otherwise emit 0,95 and produce JSON the reader rejects.
        public void Field(string name, double value)
        {
            Key(name);
            Raw(value.ToString("0.####", System.Globalization.CultureInfo.InvariantCulture));
        }
    }
}
