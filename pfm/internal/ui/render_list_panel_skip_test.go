package ui

import "testing"

// TestRenderSkipsListPanelBuildOnLimitsStatsAndCosmos pins the fix in
// render(): before it, `body := model.renderListPanel(...)` ran on EVERY
// frame and was then discarded on Stats/Limits/Cosmos — a full fleet-row
// layout built and thrown away on every frame those tabs draw, including
// every one of the Limits tab's own idle-backoff ticks (statsCadence, see
// model.go). renderListPanel is the ONLY render function that indexes
// model.rows through model.filtered; poisoning filtered with an
// out-of-range index turns "was it built" into an observable panic instead
// of something only a profiler could show.
func TestRenderSkipsListPanelBuildOnLimitsStatsAndCosmos(t *testing.T) {
	snapshot := fixtureSnapshot(120)
	snapshot.NoSky = true
	model := NewModel(snapshot)
	if len(model.rows) == 0 {
		t.Fatal("fixture has no rows — test cannot poison a real index")
	}
	model.filtered = []int{len(model.rows) + 5}

	for _, tab := range []Tab{TabStats, TabLimits, TabCosmos} {
		model.tab = tab
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("tab %v: render() panicked — renderListPanel was still built: %v", tab, r)
				}
			}()
			_ = model.render()
		}()
	}

	// Sanity check: TabChats DOES build the list panel and must panic on
	// this poisoned state, proving the fixture actually exercises
	// renderListPanel rather than passing vacuously.
	model.tab = TabChats
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("TabChats render() did not panic on a poisoned filtered index — fixture no longer exercises renderListPanel")
			}
		}()
		_ = model.render()
	}()
}
