package mid

import (
	"errors"
)

type PushOfflineMessagesFuc func(userID int64)

var pushOfflineMessages PushOfflineMessagesFuc

func RegisterPushOfflineMessagesFuc(fuc PushOfflineMessagesFuc) {
	pushOfflineMessages = fuc
}

func InvokePushOfflineMessages(userID int64) error {
	if pushOfflineMessages != nil {
		pushOfflineMessages(userID)
	}
	return errors.New("not registered")
}
