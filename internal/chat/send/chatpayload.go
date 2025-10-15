package send

type ChatPayload interface {
	GetType() string
}

type TextPayload struct {
	Text string `json:"text"`
}

type ImagePayload struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type VoicePayload struct {
	URL      string `json:"url"`
	Duration int    `json:"duration"` // 秒数
}

type FilePayload struct {
	URL      string `json:"url"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size"` // 字节
}

func (t TextPayload) GetType() string {
	return "text"
}

func (i ImagePayload) GetType() string {
	return "image"
}

func (v VoicePayload) GetType() string {
	return "voice"
}

func (f FilePayload) GetType() string {
	return "file"
}
