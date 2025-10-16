package push

import (
	"time"
)

func PushViaWS(userID int64, writeWait time.Duration, message ClientMessage) error {
	holder, ok := GetConnectionHolder(userID)
	if !ok {
		return ErrNoConn
	}
	err := WriteJSONSafe(holder, writeWait, message)
	if err != nil && err != ErrNoConn {
		RemoveConnection(userID)
	}
	return err
}

func PushViaWSWithRetry(userID int64, times int, writeWait time.Duration, message ClientMessage) error {
	var err error
	//initial try
	err = PushViaWS(userID, writeWait, message)
	if err == nil {
		return nil
	}
	if err == ErrNoConn {
		return err
	}
	//retry
	for i := 1; i < times; i++ {
		err = PushViaWS(userID, writeWait, message)
		if err == nil {
			return nil
		}
		//wait before retry
		time.Sleep(100 * time.Millisecond)
	}
	return err
}
