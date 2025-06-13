package comm

type Config struct {
	VideoWidth  int
	VideoHeight int
	MimeType    string
	Orientation int
	UseAdb      bool
	AdbConnect  bool
	SecurityKey string
	Password    string
	MaxSize     int
}
