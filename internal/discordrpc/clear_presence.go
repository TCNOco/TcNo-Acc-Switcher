package discordrpc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"

	"github.com/hugolgst/rich-go/ipc"
)

// clearPresenceDiscord tells Discord to remove the game's rich presence before closing IPC.
func clearPresenceDiscord() error {
	payload, err := json.Marshal(map[string]interface{}{
		"cmd": "SET_ACTIVITY",
		"args": map[string]interface{}{
			"pid":      os.Getpid(),
			"activity": nil,
		},
		"nonce": randomNonceHex(),
	})
	if err != nil {
		return err
	}
	_ = ipc.Send(1, string(payload))
	return nil
}

func randomNonceHex() string {
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	if err != nil {
		return fmt.Sprintf("%d-%d", os.Getpid(), os.Getppid())
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:])
}
