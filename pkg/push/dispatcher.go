package push

func Dispatch(msg PushMessage) error{
	for _,uid := range msg.TargetID {
		err := pushViaWebSocket(uid, msg)
		if err != nil {
			// log error.Later support offline push via other channels
			continue
		}
	}
	return nil
}