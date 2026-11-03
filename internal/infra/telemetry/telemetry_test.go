package telemetry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTraceSampler(t *testing.T) {
	for _, name := range []string{"always_on", "always_off", "parentbased_always_on", "parentbased_always_off", "traceidratio", "parentbased_traceidratio"} {
		sampler, err := traceSampler(name, "0.5")
		require.NoError(t, err)
		require.NotNil(t, sampler)
	}
	for _, arg := range []string{"-1", "2", "bad", "NaN", "+Inf"} {
		_, err := traceSampler("traceidratio", arg)
		require.Error(t, err)
	}
	_, err := traceSampler("bad", "")
	require.Error(t, err)
}
