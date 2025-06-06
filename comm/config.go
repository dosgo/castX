package comm

type Config struct {
	ScreenWidth  int
	ScreenHeight int
	VideoWidth   int
	VideoHeight  int
	MimeType     string
	Orientation  int
	UseAdb       bool
	AdbConnect   bool
	SecurityKey  string
	Password     string
	MaxSize      int
}
