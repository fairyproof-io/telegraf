package nsq

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

	server := []string{testutil.GetLocalHost() + ":4150"}
	s, _ := serializers.NewInfluxSerializer()
	n := &NSQ{
		Server:     server[0],
		Topic:      "telegraf",
		serializer: s,
	}

	// Verify that we can connect to the NSQ daemon
	err := n.Connect()
	require.NoError(t, err)

	// Verify that we can successfully write data to the NSQ daemon
	err = n.Write(testutil.MockMetrics())
	require.NoError(t, err)
}
