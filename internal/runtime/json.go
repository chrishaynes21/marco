package runtime

// JSON marshalling for the host bridge (see spec/Hosts.md). A foreign call's
// Input is encoded to a JSON-able Go value sent to an out-of-process host; the
// host's response data is decoded back into a Value. The mapping:
//
//	absent  ↔ null
//	text    ↔ string
//	number  ↔ float64 (JSON number)
//	boolean ↔ bool
//	set     ↔ object
//	list    ↔ array
//	error   ↔ {"error": "<message>"}
//
// Containers (queue/feed/channel) are not crossable and encode as null.

// JSONFromValue converts a Value into a json.Marshal-able Go value.
func JSONFromValue(v Value) any {
	switch v.tag {
	case tagText:
		return v.s
	case tagNumber:
		return v.n
	case tagBoolean:
		return v.b
	case tagSet:
		m := map[string]any{}
		for _, f := range v.set.SnapshotFields() {
			m[f.Name] = JSONFromValue(f.Value)
		}
		return m
	case tagList:
		arr := []any{}
		for _, e := range v.lst.Snapshot() {
			arr = append(arr, JSONFromValue(e))
		}
		return arr
	case tagError:
		return map[string]any{"error": v.err.Message}
	default: // tagAbsent and non-crossable containers
		return nil
	}
}

// ValueFromJSON converts a decoded JSON value (as produced by encoding/json into
// interface{}) back into a Value. A single-key {"error": "<msg>"} object decodes
// to an error Value; any other object decodes to a set.
func ValueFromJSON(x any) Value {
	switch t := x.(type) {
	case nil:
		return Absent()
	case string:
		return Text(t)
	case float64:
		return Number(t)
	case bool:
		return Bool(t)
	case map[string]any:
		if len(t) == 1 {
			if msg, ok := t["error"]; ok {
				if s, ok := msg.(string); ok {
					return ErrVal(&Err{Message: s})
				}
			}
		}
		s := NewSet()
		for k, val := range t {
			s.Put(k, ValueFromJSON(val))
		}
		return SetVal(s)
	case []any:
		l := NewList()
		for _, e := range t {
			l.Append(ValueFromJSON(e))
		}
		return ListVal(l)
	default:
		return Absent()
	}
}
