package ux

import "testing"

func TestReplayOpensWithTheBuriedLayer(t *testing.T) {
	var live Grid
	if live.ShowBuriedFood() {
		t.Error("the buried layer is on before anything switched it on; " +
			"a live run would open with a wash over the whole world")
	}

	var rep Grid
	rep.applyReplayViewDefaults()
	if !rep.ShowBuriedFood() {
		t.Error("a replay did not open with the buried layer visible")
	}

	// The toggle still works afterwards: opening it on must not pin it on.
	rep.showBuriedFood = false
	if rep.ShowBuriedFood() {
		t.Error("the buried layer could not be switched back off")
	}
}
