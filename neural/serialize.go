package neural

import (
	"encoding/binary"
	"fmt"
	"math"

	d "github.com/Zebbeni/protozoa/decision"
)

// A net is stored as bytes rather than as the digit string a decision tree
// uses, because what has to survive is a few hundred float64s.
//
// Every weight is stored at full width on purpose. A float32 would halve the
// size of a recording and break the thing recordings exist for: a restored
// organism whose weights are rounded takes a different branch somewhere, and
// the replay diverges from the run it recorded.

// serialVersion is written first so a later change to the layout can be told
// from the current one rather than being read as garbage weights.
const serialVersion = 1

// Encode writes the net as bytes for a snapshot.
func (n *Net) Encode() []byte {
	buf := make([]byte, 0, 16+8*n.Weights())
	buf = binary.AppendUvarint(buf, serialVersion)

	buf = binary.AppendUvarint(buf, uint64(len(n.inputs)))
	for _, c := range n.inputs {
		buf = binary.AppendUvarint(buf, uint64(c))
	}
	buf = binary.AppendUvarint(buf, uint64(len(n.outputs)))
	for _, a := range n.outputs {
		buf = binary.AppendUvarint(buf, uint64(a))
	}

	buf = binary.AppendUvarint(buf, uint64(len(n.layers)))
	for _, l := range n.layers {
		buf = binary.AppendUvarint(buf, uint64(len(l.biases)))
		for j := range l.biases {
			buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(l.biases[j]))
			for _, w := range l.weights[j] {
				buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(w))
			}
		}
	}
	return buf
}

// Decode rebuilds a net from Encode's bytes.
//
// The weight counts are not stored per unit: they are implied by the layer
// before, which is what makes a malformed net detectable rather than silently
// reshaped. A net that does not read back to a legal shape is an error, since
// the alternative is restoring an organism whose brain is not the one that was
// recorded.
func Decode(data []byte) (*Net, error) {
	r := &reader{data: data}

	version, err := r.uvarint()
	if err != nil {
		return nil, err
	}
	if version != serialVersion {
		return nil, fmt.Errorf("neural: unknown net format %d", version)
	}

	net := &Net{}
	nIn, err := r.uvarint()
	if err != nil {
		return nil, err
	}
	for i := uint64(0); i < nIn; i++ {
		code, err := r.uvarint()
		if err != nil {
			return nil, err
		}
		net.inputs = append(net.inputs, d.Condition(code))
	}

	nOut, err := r.uvarint()
	if err != nil {
		return nil, err
	}
	for i := uint64(0); i < nOut; i++ {
		code, err := r.uvarint()
		if err != nil {
			return nil, err
		}
		net.outputs = append(net.outputs, d.Action(code))
	}

	nLayers, err := r.uvarint()
	if err != nil {
		return nil, err
	}
	if nLayers == 0 {
		return nil, fmt.Errorf("neural: a net needs at least an output layer")
	}
	net.layers = make([]layer, nLayers)
	for li := range net.layers {
		units, err := r.uvarint()
		if err != nil {
			return nil, err
		}
		prev := net.inputWidth(li)
		l := layer{
			weights: make([][]float64, units),
			biases:  make([]float64, units),
		}
		for j := uint64(0); j < units; j++ {
			if l.biases[j], err = r.float(); err != nil {
				return nil, err
			}
			l.weights[j] = make([]float64, prev)
			for i := 0; i < prev; i++ {
				if l.weights[j][i], err = r.float(); err != nil {
					return nil, err
				}
			}
		}
		net.layers[li] = l
	}

	if got, want := len(net.layers[len(net.layers)-1].biases), len(net.outputs); got != want {
		return nil, fmt.Errorf("neural: output layer has %d units for %d actions", got, want)
	}
	if r.off != len(r.data) {
		return nil, fmt.Errorf("neural: %d bytes left over", len(r.data)-r.off)
	}
	return net, nil
}

type reader struct {
	data []byte
	off  int
}

func (r *reader) uvarint() (uint64, error) {
	v, n := binary.Uvarint(r.data[r.off:])
	if n <= 0 {
		return 0, fmt.Errorf("neural: truncated count at byte %d", r.off)
	}
	r.off += n
	return v, nil
}

func (r *reader) float() (float64, error) {
	if r.off+8 > len(r.data) {
		return 0, fmt.Errorf("neural: truncated weight at byte %d", r.off)
	}
	bits := binary.LittleEndian.Uint64(r.data[r.off:])
	r.off += 8
	return math.Float64frombits(bits), nil
}
