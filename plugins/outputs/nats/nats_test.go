package nats

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

	server := []string{"nats://" + testutil.GetLocalHost() + ":4222"}
	s, _ := serializers.NewInfluxSerializer()
	n := &NATS{
		Servers:    server,
		Name:       "telegraf",
		Subject:    "telegraf",
		serializer: s,
	}

	// Verify that we can connect to the NATS daemon
	err := n.Connect()
	require.NoError(t, err)

	// Verify that we can successfully write data to the NATS daemon
	err = n.Write(testutil.MockMetrics())
	require.NoError(t, err)
}
