package mqtt

import (
	"testing"

	"github.com/fairyproof-io/telegraf/plugins/serializers"
	"github.com/fairyproof-io/telegraf/testutil"

	"github.com/stretchr/testify/require"
)

func TestConnectAndWriteIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	var url = testutil.GetLocalHost() + ":1883"
	s, _ := serializers.NewInfluxSerializer()
	m := &MQTT{
		Servers:    []string{url},
		serializer: s,
		KeepAlive:  30,
	}

	// Verify that we can connect to the MQTT broker
	err := m.Connect()
	require.NoError(t, err)

	// Verify that we can successfully write data to the mqtt broker
	err = m.Write(testutil.MockMetrics())
	require.NoError(t, err)
}
