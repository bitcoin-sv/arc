package fees

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateFee(t *testing.T) {
	type args struct {
		satsPerKB             uint64
		numberOfInputs        uint64
		numberOfPayingOutputs uint64
	}
	tests := []struct {
		name string
		args args
		want uint64
	}{
		{
			name: "1 input, 1 output",
			args: args{
				satsPerKB:             50,
				numberOfInputs:        1,
				numberOfPayingOutputs: 1,
			},
			want: uint64(10),
		},
		{
			name: "1 input, 2 output",
			args: args{
				satsPerKB:             50,
				numberOfInputs:        1,
				numberOfPayingOutputs: 2,
			},
			want: uint64(12),
		},
		{
			name: "1 input, 51 output",
			args: args{
				satsPerKB:             50,
				numberOfInputs:        1,
				numberOfPayingOutputs: 51,
			},
			want: uint64(95),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fee := EstimateFee(tt.args.satsPerKB, tt.args.numberOfInputs, tt.args.numberOfPayingOutputs)
			assert.Equalf(t, tt.want, fee, "EstimateFee(%d, %d, %d) => %d", tt.args.satsPerKB, tt.args.numberOfInputs, tt.args.numberOfPayingOutputs, fee)
		})
	}
}
