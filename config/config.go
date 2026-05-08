package config

var (
	DbPath         string
	LogLevel       string
	QiniuAccessKey string
	QiniuSecretKey string
	QiniuBucket   string
	PasswordHash  string
)

func SetLogPath(path string) {
	LogPath = path
}

func GetLogPath() string {
	if LogPath != "" {
		return LogPath
	}
	return "/tmp/assh.log"
}

func SetDbPath(path string) {
	DbPath = path
}

func GetDbPath() string {
	if DbPath != "" {
		return DbPath
	}
	return DbFile
}

func SetLogLevel(level string) {
	LogLevel = level
}

func GetLogLevel() string {
	return LogLevel
}

func SetQiniuConfig(accessKey, secretKey, bucket string) {
	QiniuAccessKey = accessKey
	QiniuSecretKey = secretKey
	QiniuBucket = bucket
}

func GetQiniuConfig() (accessKey, secretKey, bucket string) {
	return QiniuAccessKey, QiniuSecretKey, QiniuBucket
}

func HasPassword() bool {
	return PasswordHash != ""
}

func SetPasswordHash(hash string) {
	PasswordHash = hash
}

func GetPasswordHash() string {
	return PasswordHash
}