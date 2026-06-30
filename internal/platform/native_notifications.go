package platform

import (
	"log"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

var nativeNotifier struct {
	sync.RWMutex
	service *notifications.NotificationService
}

func SetNativeNotifier(service *notifications.NotificationService) {
	nativeNotifier.Lock()
	nativeNotifier.service = service
	nativeNotifier.Unlock()
}

func NotifyNative(id, title, body string, data map[string]interface{}) {
	id = strings.TrimSpace(id)
	title = strings.TrimSpace(title)
	if id == "" || title == "" {
		return
	}
	nativeNotifier.RLock()
	service := nativeNotifier.service
	nativeNotifier.RUnlock()
	if service == nil {
		return
	}
	ok, err := service.CheckNotificationAuthorization()
	if err != nil || !ok {
		return
	}
	if err := service.SendNotification(notifications.NotificationOptions{
		ID:    id,
		Title: title,
		Body:  strings.TrimSpace(body),
		Data:  data,
	}); err != nil {
		log.Printf("native notification: %v", err)
	}
}
