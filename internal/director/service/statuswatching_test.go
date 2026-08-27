package service

import (
	"encoding/json"
	"strings"
	"testing"
)

// A person asking what the Director is doing is asking, among other things, whether it is
// watching them. These hold that the status surface answers it — always, and without the answer
// being the thing that makes it true.
//
// See ADR-093.

func TestStatusAlwaysSaysWhetherMarcoIsWatching(t *testing.T) {
	rt := newFakeRuntime()
	_, dir := serve(t, rt)
	c := dial(t, dir)

	st, err := c.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Watching.Watching {
		t.Fatal("a Director nobody asked to watch reported that it is watching")
	}

	// AND IT IS IN THE JSON WHEN THE ANSWER IS NO, which is the whole reason it is a value
	// and not a pointer. A front-end that saw no field would have to guess between "not
	// watching" and "a Director too old to say", and those are not the same answer to a
	// question about being observed. Making it a *AmbientView must fail this.
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"watching":{`) {
		t.Errorf("status JSON drops the watching field when the answer is no:\n%s", raw)
	}
}

func TestStatusCarriesWhatWatchingHasNoticed(t *testing.T) {
	rt := newFakeRuntime()
	rt.ambient = AmbientView{
		Watching: true, Application: "settings", Places: 3, Transitions: 2, Samples: 41,
	}
	_, dir := serve(t, rt)
	c := dial(t, dir)

	st, err := c.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !st.Watching.Watching {
		t.Fatal("status did not report that Marco is watching")
	}
	if st.Watching.Places != 3 || st.Watching.Transitions != 2 {
		t.Errorf("counts did not survive the round trip: %+v", st.Watching)
	}
}
