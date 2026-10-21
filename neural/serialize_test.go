package neural

import (
	"testing"

	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/simrand"
)

// grown is a net with some history behind it, which is what a snapshot
// actually holds.
func grown(t *testing.T, seed uint64, generations int) *Net {
	t.Helper()
	rng := simrand.New(seed)
	net := Starter(d.ActChemosynthesis)
	for i := 0; i < generations; i++ {
		net = Inherit(rng, net)
	}
	return net
}

// TestANetSurvivesItsOwnEncoding is what a seek rests on: an organism
// restored from a snapshot has to behave identically to the one that was
// written, which means every weight has to come back EXACTLY — a rounded
// weight takes a different branch somewhere and the replay diverges from the
// run it recorded.
func TestANetSurvivesItsOwnEncoding(t *testing.T) {
	shapeGlobals(t, 6, 2)
	for seed := uint64(1); seed <= 8; seed++ {
		net := grown(t, seed, 80)
		back, err := Decode(net.Encode())
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		checkShape(t, back, "after decoding")

		if len(back.Inputs()) != len(net.Inputs()) {
			t.Fatalf("seed %d: %d inputs decoded from %d", seed, len(back.Inputs()), len(net.Inputs()))
		}
		for i, c := range net.Inputs() {
			if back.Inputs()[i] != c {
				t.Errorf("seed %d: input %d decoded as %v, want %v", seed, i, back.Inputs()[i], c)
			}
		}
		for i, a := range net.Outputs() {
			if back.Outputs()[i] != a {
				t.Errorf("seed %d: output %d decoded as %v, want %v", seed, i, back.Outputs()[i], a)
			}
		}
		for li := range net.layers {
			for j := range net.layers[li].biases {
				if back.layers[li].biases[j] != net.layers[li].biases[j] {
					t.Errorf("seed %d: layer %d bias %d decoded as %v, want %v",
						seed, li, j, back.layers[li].biases[j], net.layers[li].biases[j])
				}
				for i, w := range net.layers[li].weights[j] {
					if back.layers[li].weights[j][i] != w {
						t.Errorf("seed %d: layer %d weight %d,%d decoded as %v, want %v",
							seed, li, j, i, back.layers[li].weights[j][i], w)
					}
				}
			}
		}
	}
}

// TestADecodedNetChoosesTheSameAction states the round trip as the property
// that matters, rather than as a field-by-field comparison: the restored
// organism does what the recorded one did.
func TestADecodedNetChoosesTheSameAction(t *testing.T) {
	shapeGlobals(t, 6, 2)
	for seed := uint64(1); seed <= 20; seed++ {
		net := grown(t, seed, 60)
		back, err := Decode(net.Encode())
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		for _, eval := range []func(d.Condition) bool{alwaysTrue, alwaysFalse} {
			if got, want := back.Choose(eval), net.Choose(eval); got != want {
				t.Errorf("seed %d: the decoded net chose %v, the original %v", seed, got, want)
			}
			for j, v := range back.LastOutputs() {
				if v != net.LastOutputs()[j] {
					t.Errorf("seed %d: decoded output %d is %v, the original %v",
						seed, j, v, net.LastOutputs()[j])
				}
			}
		}
	}
}

// TestEncodingIsStable: the same net encodes to the same bytes, so a snapshot
// written twice from one state is one file rather than two.
func TestEncodingIsStable(t *testing.T) {
	shapeGlobals(t, 6, 2)
	net := grown(t, 99, 50)
	if string(net.Encode()) != string(net.Encode()) {
		t.Error("encoding the same net twice gave different bytes")
	}
}

// TestMalformedNetsAreRefused: a net that does not read back to a legal shape
// has to be an error, since the alternative is an organism quietly restored
// with a brain that is not the one recorded.
func TestMalformedNetsAreRefused(t *testing.T) {
	shapeGlobals(t, 6, 2)
	good := grown(t, 5, 40).Encode()

	cases := map[string][]byte{
		"empty":            {},
		"truncated":        good[:len(good)/2],
		"trailing bytes":   append(append([]byte(nil), good...), 0x01, 0x02),
		"unknown version":  append([]byte{0x7f}, good[1:]...),
		"no layers at all": {serialVersion, 0, 0, 0},
	}
	for name, data := range cases {
		if _, err := Decode(data); err == nil {
			t.Errorf("%s decoded without an error", name)
		}
	}
}

// TestTheRecordedSizeIsWorthKnowing is not an assertion about a number — it
// prints what one organism costs a recording, because a net is stored as
// float64 weights and a snapshot holds one per organism. It is the figure to
// check against `max_replay_size_mb` before a long run on nets.
func TestTheRecordedSizeIsWorthKnowing(t *testing.T) {
	shapeGlobals(t, 8, 2)
	net := grown(t, 1, 400)
	t.Logf("after 400 generations: %d inputs, %d outputs, layers %v, %d weights, %d bytes encoded",
		len(net.Inputs()), len(net.Outputs()), net.LayerSizes(), net.Weights(), len(net.Encode()))
}
